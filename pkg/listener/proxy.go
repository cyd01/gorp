package listener

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"time"
)

type forwardProxy struct {
	transport http.RoundTripper
	timeout   time.Duration
}

func NewProxyHandler(timeout time.Duration) http.Handler {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &forwardProxy{
		transport: &http.Transport{Proxy: nil},
		timeout:   timeout,
	}
}

func (p *forwardProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			outbound.URL.Scheme = r.URL.Scheme
			outbound.URL.Host = r.URL.Host
			outbound.Host = r.Host
		},
	}
	proxy.ServeHTTP(w, r)
}

func (p *forwardProxy) serveConnect(w http.ResponseWriter, r *http.Request) {
	if r.Host == "" {
		http.Error(w, "CONNECT requires a host", http.StatusBadRequest)
		return
	}
	upstream, err := net.DialTimeout("tcp", r.Host, p.timeout)
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
