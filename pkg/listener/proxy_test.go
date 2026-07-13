package listener

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProxyHandlerForwardsHTTPRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/through-proxy" {
			t.Errorf("path = %q, want /through-proxy", r.URL.Path)
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	proxy := httptest.NewServer(NewProxyHandler(time.Second, false))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err := client.Get(upstream.URL + "/through-proxy")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
}

func TestProxyHandlerForwardsThroughCascade(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "cascade-ok")
	}))
	defer upstream.Close()

	cascade := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Scheme == "" || r.URL.Host == "" {
			http.Error(w, "expected absolute URL", http.StatusBadRequest)
			return
		}
		response, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		for key, values := range response.Header {
			w.Header()[key] = values
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	defer cascade.Close()

	handler, err := NewProxyHandlerWithProxy(time.Second, false, nil, &CascadeProxy{URL: cascade.URL})
	if err != nil {
		t.Fatal(err)
	}
	outer := httptest.NewServer(handler)
	defer outer.Close()
	outerURL, err := url.Parse(outer.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(outerURL)}}
	response, err := client.Get(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "cascade-ok" {
		t.Fatalf("body = %q, want cascade-ok", body)
	}
}

func TestProxyHandlerCascadeHostnameMatchAndException(t *testing.T) {
	const directBody = "direct"
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, directBody)
	}))
	defer direct.Close()

	cascade := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Hostname() != "api.example.test" {
			t.Errorf("cascade destination host = %q, want api.example.test", r.URL.Hostname())
		}
		_, _ = io.WriteString(w, "via-cascade")
	}))
	defer cascade.Close()

	handler, err := NewProxyHandlerWithProxy(time.Second, false, nil, &CascadeProxy{
		URL:     cascade.URL,
		Hosts:   []string{"*.example.test", "127.0.0.1"},
		NoProxy: []string{"127.0.0.1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	outer := httptest.NewServer(handler)
	defer outer.Close()
	outerURL, err := url.Parse(outer.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(outerURL)}}

	response, err := client.Get("http://api.example.test/path")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "via-cascade" {
		t.Fatalf("matched response = %q, want via-cascade", body)
	}

	response, err = client.Get(direct.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != directBody {
		t.Fatalf("excepted response = %q, want %q", body, directBody)
	}
}

func TestCascadeProxyHostnamePatternMatching(t *testing.T) {
	proxy, err := newCascadeProxy(time.Second, CascadeProxy{
		URL:     "http://proxy.example.test:8080",
		Hosts:   []string{"*.example.com", "10.?.0.[1-3]"},
		NoProxy: []string{"private.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		host string
		want bool
	}{
		{host: "api.example.com", want: true},
		{host: "API.EXAMPLE.COM.", want: true},
		{host: "private.example.com", want: false},
		{host: "other.test", want: false},
		{host: "10.2.0.3", want: true},
	}
	for _, test := range tests {
		if got := proxy.matchesCascade(test.host); got != test.want {
			t.Errorf("matchesCascade(%q) = %t, want %t", test.host, got, test.want)
		}
	}
}

func TestProxyHandlerRequiresBasicAuthentication(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "authenticated")
	}))
	defer upstream.Close()

	proxy := httptest.NewServer(NewProxyHandler(time.Second, false, map[string]string{"alice": "secret"}))
	defer proxy.Close()

	response, err := http.Get(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusProxyAuthRequired)
	}
	if response.Header.Get("Proxy-Authenticate") == "" {
		t.Fatal("missing Proxy-Authenticate header")
	}

	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxyURL.User = url.UserPassword("alice", "secret")
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err = client.Get(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "authenticated" {
		t.Fatalf("body = %q, want authenticated", body)
	}
}

func TestProxyHandlerConnectsToRequestedHost(t *testing.T) {
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoListener.Close()
	go func() {
		connection, acceptErr := echoListener.Accept()
		if acceptErr == nil {
			defer connection.Close()
			_, _ = io.Copy(connection, connection)
		}
	}()

	proxy := httptest.NewServer(NewProxyHandler(time.Second, false))
	defer proxy.Close()
	proxyAddress := strings.TrimPrefix(proxy.URL, "http://")
	client, err := net.Dial("tcp", proxyAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, _ = fmt.Fprintf(client, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", echoListener.Addr(), echoListener.Addr())

	reader := bufio.NewReader(client)
	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if response != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("response = %q", response)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("tunnel response = %q", got)
	}
}

func TestProxyHandlerConnectsThroughHTTPServer(t *testing.T) {
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoListener.Close()
	go func() {
		connection, acceptErr := echoListener.Accept()
		if acceptErr == nil {
			defer connection.Close()
			_, _ = io.Copy(connection, connection)
		}
	}()

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: NewProxyHandler(time.Second, false)}
	go server.Serve(proxyListener)
	defer server.Close()

	client, err := net.Dial("tcp", proxyListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, _ = fmt.Fprintf(client, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", echoListener.Addr(), echoListener.Addr())
	reader := bufio.NewReader(client)
	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if response != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("response = %q", response)
	}
}

func TestProxyHandlerOverTLS(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	proxy := httptest.NewUnstartedServer(NewProxyHandler(time.Second, false))
	proxy.StartTLS()
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxyTransport := proxy.Client().Transport.(*http.Transport).Clone()
	proxyTransport.Proxy = http.ProxyURL(proxyURL)
	client := &http.Client{Transport: proxyTransport}
	response, err := client.Get(upstream.URL + "/through-tls-proxy")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
}
