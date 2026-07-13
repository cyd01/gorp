package listener

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/cyd01/gorp/pkg/helper"
	xproxy "golang.org/x/net/proxy"
)

type forwardProxy struct {
	transport http.RoundTripper
	timeout   time.Duration
	users     map[string]string
	log       bool
	cascade   *cascadeProxy
}

type CascadeProxy struct {
	URL      string
	Username string
	Password string
}

type cascadeProxy struct {
	url      *url.URL
	timeout  time.Duration
	username string
	password string
}

func NewProxyHandler(timeout time.Duration, log bool, configuredUsers ...map[string]string) http.Handler {
	handler, _ := newProxyHandler(timeout, log, configuredUsers, nil)
	return handler
}

func NewProxyHandlerWithProxy(timeout time.Duration, log bool, users map[string]string, proxy *CascadeProxy) (http.Handler, error) {
	var configured *cascadeProxy
	var err error
	if proxy != nil {
		configured, err = newCascadeProxy(timeout, *proxy)
		if err != nil {
			return nil, err
		}
	}
	return newProxyHandler(timeout, log, []map[string]string{users}, configured)
}

func newProxyHandler(timeout time.Duration, log bool, configuredUsers []map[string]string, configuredProxy *cascadeProxy) (http.Handler, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	proxy := configuredProxy
	transport := &http.Transport{Proxy: nil}
	if proxy != nil && proxy.url.Scheme == "socks5" {
		dialer, err := newSOCKS5Dialer(proxy)
		if err != nil {
			return nil, err
		}
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.Dial(network, address)
		}
	} else if proxy != nil {
		transport.Proxy = http.ProxyURL(proxy.url)
	}
	return &forwardProxy{
		transport: transport,
		timeout:   timeout,
		users:     firstUsers(configuredUsers),
		log:       log,
		cascade:   proxy,
	}, nil
}

func NewProxyTLSServer(address, cert, key string, timeout time.Duration, log bool, minVersion ...string) (*http.Server, error) {
	return newProxyTLSServer(address, cert, key, timeout, log, minVersion, nil, nil)
}

func NewProxyTLSServerWithUsers(address, cert, key string, timeout time.Duration, log bool, users map[string]string, minVersion ...string) (*http.Server, error) {
	return newProxyTLSServer(address, cert, key, timeout, log, minVersion, users, nil)
}

func NewProxyTLSServerWithProxy(address, cert, key string, timeout time.Duration, log bool, users map[string]string, proxy *CascadeProxy, minVersion ...string) (*http.Server, error) {
	return newProxyTLSServer(address, cert, key, timeout, log, minVersion, users, proxy)
}

func newProxyTLSServer(address, cert, key string, timeout time.Duration, log bool, minVersion []string, users map[string]string, configuredProxy *CascadeProxy) (*http.Server, error) {
	configuredMinVersion := ""
	if len(minVersion) > 0 {
		configuredMinVersion = minVersion[0]
	}
	tlsConfig, err := helper.BuildTLSConfigForDownstreamWithMinVersion(cert, key, "", "", "", configuredMinVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to build proxy TLS config: %w", err)
	}
	handler, err := NewProxyHandlerWithProxy(timeout, log, users, configuredProxy)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:      address,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}, nil
}

func newCascadeProxy(timeout time.Duration, configured ...CascadeProxy) (*cascadeProxy, error) {
	if len(configured) == 0 || configured[0].URL == "" {
		return nil, nil
	}
	parsed, err := url.Parse(configured[0].URL)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid cascade proxy URL %q", configured[0].URL)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "socks5" {
		return nil, fmt.Errorf("unsupported cascade proxy scheme %q", parsed.Scheme)
	}
	username, password := configured[0].Username, configured[0].Password
	if parsed.User == nil && username != "" {
		parsed.User = url.UserPassword(username, password)
	}
	return &cascadeProxy{url: parsed, timeout: timeout, username: username, password: password}, nil
}

func newSOCKS5Dialer(proxy *cascadeProxy) (xproxy.Dialer, error) {
	var auth *xproxy.Auth
	if proxy.username != "" {
		auth = &xproxy.Auth{User: proxy.username, Password: proxy.password}
	}
	return xproxy.SOCKS5("tcp", proxy.url.Host, auth, &net.Dialer{Timeout: proxy.timeout})
}

func dialSOCKS5Proxy(ctx context.Context, proxy *cascadeProxy, target string) (net.Conn, error) {
	_ = ctx
	dialer, err := newSOCKS5Dialer(proxy)
	if err != nil {
		return nil, err
	}
	return dialer.Dial("tcp", target)
}

func dialHTTPProxy(ctx context.Context, proxy *cascadeProxy, target string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: proxy.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", proxy.url.Host)
	if err != nil {
		return nil, err
	}
	if proxy.url.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: proxy.url.Hostname(), MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	request := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", target, target)
	if proxy.url.User != nil {
		credentials := proxy.url.User.Username()
		password, _ := proxy.url.User.Password()
		encoded := base64.StdEncoding.EncodeToString([]byte(credentials + ":" + password))
		request += "Proxy-Authorization: Basic " + encoded + "\r\n"
	}
	request += "\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("cascade proxy CONNECT returned %s", response.Status)
	}
	if reader.Buffered() > 0 {
		return &bufferedProxyConn{Conn: conn, reader: reader}, nil
	}
	return conn, nil
}

type bufferedProxyConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedProxyConn) Read(data []byte) (int, error) {
	return c.reader.Read(data)
}

func (p *forwardProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(p.users) > 0 && !p.authenticate(w, r) {
		return
	}
	if p.log {
		log.Printf("[PROXY][%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)
	}
	if r.Method == http.MethodConnect {
		p.serveConnect(w, r)
		return
	}
	if r.URL == nil || r.URL.Scheme == "" || r.URL.Host == "" {
		http.Error(w, "proxy requests must use an absolute URL", http.StatusBadRequest)
		return
	}

	proxy := &httputil.ReverseProxy{
		Transport: p.transport,
		Director: func(outbound *http.Request) {
			outbound.Header.Del("Proxy-Authorization")
			outbound.URL.Scheme = r.URL.Scheme
			outbound.URL.Host = r.URL.Host
			outbound.Host = r.Host
		},
	}
	proxy.ServeHTTP(w, r)
}

func (p *forwardProxy) dialCascade(ctx context.Context, target string) (net.Conn, error) {
	if p.cascade == nil {
		return (&net.Dialer{Timeout: p.timeout}).DialContext(ctx, "tcp", target)
	}
	if p.cascade.url.Scheme == "socks5" {
		return dialSOCKS5Proxy(ctx, p.cascade, target)
	}
	return dialHTTPProxy(ctx, p.cascade, target)
}

func (p *forwardProxy) authenticate(w http.ResponseWriter, r *http.Request) bool {
	username, password, ok := parseProxyBasicAuth(r.Header.Get("Proxy-Authorization"))
	if ok {
		if expected, found := p.users[username]; found && expected == password {
			return true
		}
	}
	w.Header().Set("Proxy-Authenticate", `Basic realm="proxy"`)
	http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
	return false
}

func parseProxyBasicAuth(value string) (string, string, bool) {
	const prefix = "Basic "
	if len(value) < len(prefix) || value[:len(prefix)] != prefix {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(value[len(prefix):])
	if err != nil {
		return "", "", false
	}
	credentials := string(decoded)
	for index := 0; index < len(credentials); index++ {
		if credentials[index] == ':' {
			return credentials[:index], credentials[index+1:], true
		}
	}
	return "", "", false
}

func firstUsers(configuredUsers []map[string]string) map[string]string {
	if len(configuredUsers) == 0 {
		return nil
	}
	return configuredUsers[0]
}

func (p *forwardProxy) serveConnect(w http.ResponseWriter, r *http.Request) {
	if r.Host == "" {
		http.Error(w, "CONNECT requires a host", http.StatusBadRequest)
		return
	}
	upstream, err := p.dialCascade(r.Context(), r.Host)
	if err != nil {
		http.Error(w, fmt.Sprintf("connect to %s: %v", r.Host, err), http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		upstream.Close()
		http.Error(w, "HTTP CONNECT is not supported by this listener", http.StatusNotImplemented)
		return
	}
	client, clientBuffer, err := hijacker.Hijack()
	if err != nil {
		upstream.Close()
		return
	}
	defer client.Close()
	defer upstream.Close()
	if _, err := clientBuffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := clientBuffer.Flush(); err != nil {
		return
	}

	if clientBuffer.Reader.Buffered() > 0 {
		_, _ = io.CopyN(upstream, clientBuffer, int64(clientBuffer.Reader.Buffered()))
	}
	clientToUpstream := make(chan struct{})
	go func() {
		_, _ = io.Copy(upstream, client)
		proxyCloseWrite(upstream)
		close(clientToUpstream)
	}()
	_, _ = io.Copy(client, upstream)
	proxyCloseWrite(client)
	<-clientToUpstream
}

func proxyCloseWrite(conn net.Conn) {
	if closeWriter, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = closeWriter.CloseWrite()
	}
}

var _ http.Handler = (*forwardProxy)(nil)
