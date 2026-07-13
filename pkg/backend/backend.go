package backend

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cyd01/gorp/pkg/metrics"
)

// ModifyResponse is a function that modifies an HTTP response.
type ModifyResponse func(r *http.Response) error

type Backend struct {
	Name                string
	URL                 *url.URL
	Proxy               http.Handler
	ModifyResponse      []ModifyResponse
	ConnectTimeout      time.Duration
	HeadersTimeout      time.Duration
	IdleTimeout         time.Duration
	MaxConnections      int64
	HealthCheckPath     string
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration

	Transport         http.RoundTripper
	ActiveConnections atomic.Int64
	Healthy           atomic.Bool
	retryAfter        atomic.Int64
	failureCount      atomic.Int32
	breakerState      atomic.Int32
}

const (
	breakerClosed int32 = iota
	breakerOpen
	breakerHalfOpen
)

func (b *Backend) TryAcquire() bool {
	if b.MaxConnections <= 0 {
		return true
	}
	for {
		current := b.ActiveConnections.Load()
		if current >= b.MaxConnections {
			return false
		}
		if b.ActiveConnections.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (b *Backend) Release() {
	if b.ActiveConnections.Load() > 0 {
		active := b.ActiveConnections.Add(-1)
		metrics.SetActiveConnections(b.Name, active)
	}
}

// Connect establishes a TCP tunnel to the configured backend.
func (b *Backend) Connect(w http.ResponseWriter, r *http.Request) error {
	if !b.TryAcquire() {
		http.Error(w, "backend unavailable", http.StatusServiceUnavailable)
		return fmt.Errorf("backend %q reached max_connections=%d", b.Name, b.MaxConnections)
	}
	start := time.Now()
	status := http.StatusBadGateway
	defer func() {
		b.Release()
		metrics.ObserveRequest(b.Name, r.Method, status, time.Since(start))
	}()
	metrics.SetActiveConnections(b.Name, b.ActiveConnections.Load())

	if b.URL == nil || b.URL.Host == "" {
		metrics.ObserveBackendError(b.Name)
		return fmt.Errorf("backend %q has no valid CONNECT target", b.Name)
	}
	timeout := b.ConnectTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	upstream, err := net.DialTimeout("tcp", b.URL.Host, timeout)
	if err != nil {
		metrics.ObserveBackendError(b.Name)
		return fmt.Errorf("connect to backend %q: %w", b.Name, err)
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		upstream.Close()
		return fmt.Errorf("HTTP CONNECT is not supported by this listener")
	}
	client, clientBuffer, err := hijacker.Hijack()
	if err != nil {
		upstream.Close()
		return fmt.Errorf("hijack CONNECT client: %w", err)
	}
	defer client.Close()
	defer upstream.Close()

	if _, err := clientBuffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return err
	}
	if err := clientBuffer.Flush(); err != nil {
		return err
	}
	status = http.StatusOK

	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		_, _ = io.Copy(upstream, client)
		closeWrite(upstream)
	}()
	go func() {
		defer wait.Done()
		_, _ = io.Copy(client, upstream)
		closeWrite(client)
	}()
	wait.Wait()
	return nil
}

func closeWrite(conn net.Conn) {
	if closeWriter, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = closeWriter.CloseWrite()
	}
}

func New(name string, rawURL string, preserveHost bool, transport http.RoundTripper) (*Backend, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	b := &Backend{Name: name, URL: u}
	b.Healthy.Store(true)

	handler := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetXForwarded()
			r.SetURL(u)
			if preserveHost {
				r.Out.Host = r.In.Host
			}
		},
		ModifyResponse: func(r *http.Response) error {
			for _, f := range b.ModifyResponse {
				if err := f(r); err != nil {
					log.Printf("response modifier error: %v\n", err)
					return err
				}
			}
			if r != nil && r.StatusCode >= 500 {
				log.Printf("5xx http code error\n")
				return fmt.Errorf("5xx http code error")
			}
			b.failureCount.Store(0)
			if b.HealthCheckPath == "" && b.breakerState.CompareAndSwap(breakerHalfOpen, breakerClosed) {
				b.Healthy.Store(true)
				b.retryAfter.Store(0)
				log.Printf("circuit breaker closed for backend '%s'\n", b.Name)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			status := http.StatusBadGateway
			message := "backend unavailable"
			var netErr net.Error
			if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
				log.Printf("timeout connecting to backend '%s': %v\n", b.Name, err)
				status = http.StatusGatewayTimeout
				message = "backend timeout"
			} else {
				log.Printf("backend '%s' request error: %v\n", b.Name, err)
			}
			metrics.ObserveBackendError(b.Name)
			if b.HealthCheckPath == "" {
				switch b.breakerState.Load() {
				case breakerHalfOpen:
					b.openCircuit()
				case breakerClosed:
					if count := b.failureCount.Add(1); count >= 5 {
						b.openCircuit()
					}
				}
			}
			http.Error(w, message, status)
		},
	}
	b.Proxy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !b.TryAcquire() {
			http.Error(w, "backend unavailable", http.StatusServiceUnavailable)
			return
		}
		start := time.Now()
		responseWriter := &metricsResponseWriter{ResponseWriter: w}
		defer func() {
			b.Release()
			metrics.SetActiveConnections(b.Name, b.ActiveConnections.Load())
			status := responseWriter.status
			if status == 0 {
				status = http.StatusOK
			}
			metrics.ObserveRequest(b.Name, r.Method, status, time.Since(start))
		}()
		metrics.SetActiveConnections(b.Name, b.ActiveConnections.Load())
		handler.ServeHTTP(responseWriter, r)
	})
	return b, nil
}

type metricsResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *metricsResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *metricsResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *metricsResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *metricsResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *metricsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *metricsResponseWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (w *metricsResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(reader)
	}
	return io.Copy(w.ResponseWriter, reader)
}

func (b *Backend) Available() bool {
	if b.HealthCheckPath != "" {
		return b.Healthy.Load()
	}

	switch b.breakerState.Load() {
	case breakerClosed:
		return b.Healthy.Load()
	case breakerOpen:
		if time.Now().UnixNano() < b.retryAfter.Load() {
			return false
		}
		if b.breakerState.CompareAndSwap(breakerOpen, breakerHalfOpen) {
			b.Healthy.Store(true)
			return true
		}
	}
	return false
}

func (b *Backend) State() string {
	if !b.Healthy.Load() {
		return "open"
	}

	switch b.breakerState.Load() {
	case breakerOpen:
		return "open"
	case breakerHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

func (b *Backend) Retry() bool {
	return b.Available()
}

func (b *Backend) openCircuit() {
	b.Healthy.Store(false)
	b.failureCount.Store(0)
	b.retryAfter.Store(time.Now().Add(30 * time.Second).UnixNano())
	if b.breakerState.Swap(breakerOpen) != breakerOpen {
		log.Printf("circuit breaker opened for backend '%s'\n", b.Name)
	}
}

func (b *Backend) RunHealthChecks(ctx context.Context) {
	if b.HealthCheckPath == "" {
		return
	}
	interval := b.HealthCheckInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}

	b.checkHealth(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.checkHealth(ctx)
		}
	}
}

func (b *Backend) checkHealth(ctx context.Context) {
	if b.URL == nil {
		b.setHealth(false, "backend URL is missing")
		return
	}
	timeout := b.HealthCheckTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	target := *b.URL
	target.Path = b.HealthCheckPath
	target.RawPath = ""
	target.RawQuery = ""
	request, err := http.NewRequestWithContext(checkCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		b.setHealth(false, err.Error())
		return
	}
	response, err := (&http.Client{Transport: b.Transport}).Do(request)
	if err != nil {
		b.setHealth(false, err.Error())
		return
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		b.setHealth(false, fmt.Sprintf("health check returned HTTP %d", response.StatusCode))
		return
	}
	b.setHealth(true, "")
}

func (b *Backend) setHealth(healthy bool, reason string) {
	previous := b.Healthy.Swap(healthy)
	if healthy {
		b.failureCount.Store(0)
		b.retryAfter.Store(0)
		b.breakerState.Store(breakerClosed)
	} else {
		b.breakerState.Store(breakerOpen)
	}
	if previous != healthy {
		if healthy {
			log.Printf("health check restored backend '%s'\n", b.Name)
		} else {
			log.Printf("health check failed for backend '%s': %s\n", b.Name, reason)
		}
	}
}

var ErrUnavailableBackend = errors.New("backend unavailable")
