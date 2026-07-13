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

	proxy := httptest.NewServer(NewProxyHandler(time.Second))
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

	proxy := httptest.NewServer(NewProxyHandler(time.Second))
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
	server := &http.Server{Handler: NewProxyHandler(time.Second)}
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
