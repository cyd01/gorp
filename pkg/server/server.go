package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/quic-go/quic-go/http3"

	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/helper"
	"github.com/cyd01/gorp/pkg/httpmulti"
	"github.com/cyd01/gorp/pkg/listener"
	"github.com/cyd01/gorp/pkg/metrics"
	"github.com/cyd01/gorp/pkg/routing"
)

// Server represents a complete proxy instance.
type Server struct {
	config *config.Config
	router *routing.Router

	// Active server instances.
	httpServers      []*http.Server
	http3Servers     []*http3.Server
	httpmultiServers []*httpmulti.Server
	tcpListeners     []net.Listener

	// Graceful shutdown state.
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
	done         chan struct{}
	sigch        chan os.Signal
	errch        chan error
	shutdownOnce sync.Once
	stopped      bool
}

// New creates a new Server instance.
func New(cfg *config.Config, router *routing.Router) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		config:       cfg,
		router:       router,
		httpServers:  make([]*http.Server, 0),
		http3Servers: make([]*http3.Server, 0),
		tcpListeners: make([]net.Listener, 0),
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
		sigch:        make(chan os.Signal, 1),
		errch:        make(chan error, 1),
	}
}

// Run starts all listeners and the administration server.
func (s *Server) Run() error {
	// Setup signal handling
	signal.Notify(s.sigch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(s.sigch)

	// Start admin listener if configured
	if s.config.Admin.Enabled {
		go s.runWithRecovery("admin", func() error {
			return s.startAdminListener(
				s.config.Admin.Address,
				s.config.Admin.TLS.CertFile,
				s.config.Admin.TLS.KeyFile,
				s.config.Admin.TLS.CAFile,
				s.config.Admin.TLS.CRLFile,
				s.config.Admin.TLS.OCSPURL,
			)
		})
	}

	// Start proxy listeners
	if err := s.startListeners(); err != nil {
		return err
	}

	// Wait for shutdown signal or critical error
	go s.waitForShutdown()

	helper.Ready.SetReady(true)

	// Block until shutdown is complete
	select {
	case <-s.done:
		log.Println("Server stopped")
		return nil
	case err := <-s.errch:
		log.Printf("Critical error occurred: %v\n", err)
		s.initiateCriticalShutdown()
		return err
	}
}

// initiateCriticalShutdown initiates shutdown after a critical error.
func (s *Server) initiateCriticalShutdown() {
	s.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Initiating emergency graceful shutdown...")
	s.Shutdown(ctx)
}

// runWithRecovery executes a function and handles panics and errors.
func (s *Server) runWithRecovery(name string, fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("%s listener panic: %v", name, r)
			log.Printf("ERROR: %v\n", err)
			select {
			case s.errch <- err:
			default:
			}
			s.initiateCriticalShutdown()
		}
	}()

	if err := fn(); err != nil {
		log.Printf("ERROR: %s listener failed: %v\n", name, err)
		select {
		case s.errch <- fmt.Errorf("%s listener error: %w", name, err):
		default:
		}
		s.initiateCriticalShutdown()
	}
}

func (s *Server) waitForShutdown() {
	select {
	case <-s.done:
		return
	case sig := <-s.sigch:
		log.Printf("Received signal: %v\n", sig)
	}

	// Signal context cancellation
	s.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Initiating graceful shutdown...")
	s.Shutdown(ctx)
}

// Shutdown gracefully stops all servers
func (s *Server) Shutdown(ctx context.Context) {
	s.shutdownOnce.Do(func() {
		s.cancel()
		helper.Ready.SetReady(false)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.stopped = true

		var wg sync.WaitGroup

		// Shutdown HTTP servers
		for _, srv := range s.httpServers {
			wg.Add(1)
			go func(srv *http.Server) {
				defer wg.Done()
				log.Printf("Shutdown for HTTP server at '%s'\n", srv.Addr)
				if err := srv.Shutdown(ctx); err != nil {
					log.Printf("HTTP server shutdown error: %v\n", err)
				}
			}(srv)
		}

		// Shutdown HTTP Multi servers
		for _, srv := range s.httpmultiServers {
			wg.Add(1)
			go func(srv *httpmulti.Server) {
				defer wg.Done()
				log.Printf("Shutdown for HTTP multi server at '%s'\n", srv.Addr)
				if err := srv.Shutdown(ctx); err != nil {
					log.Printf("HTTP multi server shutdown error: %v\n", err)
				}
			}(srv)
		}

		// Shutdown HTTP/3 servers
		for _, srv := range s.http3Servers {
			wg.Add(1)
			go func(srv *http3.Server) {
				defer wg.Done()
				log.Printf("Shutdown for HTTP/3 server at '%s'\n", srv.Addr)
				if err := srv.Close(); err != nil {
					log.Printf("HTTP/3 server shutdown error: %v\n", err)
				}
			}(srv)
		}

		// Close TCP listeners
		for _, ln := range s.tcpListeners {
			wg.Add(1)
			go func(ln net.Listener) {
				defer wg.Done()
				if err := ln.Close(); err != nil {
					log.Printf("TCP listener close error: %v\n", err)
				}
			}(ln)
		}

		wg.Wait()
		close(s.done)
	})
}

func (s *Server) startAdminListener(address, cert, key, ca, crl, ocspURL string) error {
	srv, err := listener.NewAdminServer(address, cert, key, ca, crl, ocspURL, s.config, s.Shutdown)
	if err != nil {
		return err
	}
	middlewares, err := buildMiddlewareForListener(s.config.Admin.Middlewares)
	if err != nil {
		return fmt.Errorf("admin listener: %w", err)
	}
	srv.Handler = routing.ChainMiddleware(srv.Handler, middlewares...)
	if !s.registerHTTPServer(srv) {
		return nil
	}
	if cert != "" && key != "" {
		log.Printf("Admin HTTPS listener started on %s\n", address)
		err = srv.ListenAndServeTLS(cert, key)
	} else {
		log.Printf("Admin HTTP listener started on %s\n", address)
		err = srv.ListenAndServe()
	}
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) registerHTTPServer(srv *http.Server) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		_ = srv.Close()
		return false
	}
	s.httpServers = append(s.httpServers, srv)
	return true
}

func (s *Server) registerHTTP3Server(srv *http3.Server) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		_ = srv.Close()
		return false
	}
	s.http3Servers = append(s.http3Servers, srv)
	return true
}

func (s *Server) registerHTTPMultiServer(srv *httpmulti.Server) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		_ = srv.Shutdown(context.Background())
		return false
	}
	s.httpmultiServers = append(s.httpmultiServers, srv)
	return true
}

func (s *Server) registerTCPListener(ln net.Listener) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		_ = ln.Close()
		return false
	}
	s.tcpListeners = append(s.tcpListeners, ln)
	return true
}

func (s *Server) startListeners() error {
	for _, l := range s.config.Listeners {
		l := l // Capture for goroutine
		switch l.Type {
		case "https":
			go s.runWithRecovery(fmt.Sprintf("HTTPS '%s'", l.Name), func() error {
				return s.startHTTPSListener(l)
			})

		case "dynamic":
			go s.runWithRecovery(fmt.Sprintf("dynamic HTTPS '%s'", l.Name), func() error {
				return s.startDynamicHTTPSListener(l)
			})

		case "proxy":
			go s.runWithRecovery(fmt.Sprintf("proxy '%s'", l.Name), func() error {
				return s.startProxyListener(l)
			})

		case "http3":
			go s.runWithRecovery(fmt.Sprintf("HTTP/3 '%s'", l.Name), func() error {
				return s.startHTTP3Listener(l)
			})

		case "httpmulti", "multi":
			go s.runWithRecovery(fmt.Sprintf("HTTP multi '%s'", l.Name), func() error {
				return s.startHTTPMultiListener(l)
			})

		case "tcp":
			go s.runWithRecovery(fmt.Sprintf("TCP '%s'", l.Name), func() error {
				return s.startTCPListener(l)
			})

		default:
			go s.runWithRecovery(fmt.Sprintf("HTTP '%s'", l.Name), func() error {
				return s.startHTTPListener(l)
			})
		}
	}

	return nil
}

type connectionLimiter struct {
	next   http.Handler
	limit  int64
	name   string
	active atomic.Int64
}

func newConnectionLimiter(next http.Handler, limit int64, name string) http.Handler {
	return &connectionLimiter{next: next, limit: limit, name: name}
}

func (l *connectionLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if l.limit <= 0 {
		l.next.ServeHTTP(w, r)
		return
	}
	for {
		current := l.active.Load()
		if current >= l.limit {
			http.Error(w, "listener is at capacity", http.StatusServiceUnavailable)
			return
		}
		if l.active.CompareAndSwap(current, current+1) {
			break
		}
	}
	defer l.active.Add(-1)
	l.next.ServeHTTP(w, r)
}

type requestBodyLimiter struct {
	next  http.Handler
	limit int64
}

func newRequestBodyLimiter(next http.Handler, limit int64) http.Handler {
	if limit <= 0 {
		return next
	}
	return &requestBodyLimiter{next: next, limit: limit}
}

func (l *requestBodyLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if l.limit <= 0 || r == nil || r.Body == nil {
		l.next.ServeHTTP(w, r)
		return
	}
	if r.ContentLength > l.limit {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if r.ContentLength == -1 {
		l.next.ServeHTTP(w, r)
		return
	}
	l.next.ServeHTTP(w, r)
}

type limitedListener struct {
	net.Listener
	limit  int64
	active atomic.Int64
}

func newLimitedListener(ln net.Listener, limit int64) net.Listener {
	if limit <= 0 {
		return ln
	}
	return &limitedListener{Listener: ln, limit: limit}
}

func (l *limitedListener) Accept() (net.Conn, error) {
	for {
		current := l.active.Load()
		if current >= l.limit {
			return nil, errors.New("listener max_connections reached")
		}
		if l.active.CompareAndSwap(current, current+1) {
			break
		}
	}
	conn, err := l.Listener.Accept()
	if err != nil {
		l.active.Add(-1)
		return nil, err
	}
	return &limitedConn{Conn: conn, onClose: func() { l.active.Add(-1) }}, nil
}

type limitedConn struct {
	net.Conn
	onClose func()
}

func (c *limitedConn) Close() error {
	err := c.Conn.Close()
	if c.onClose != nil {
		c.onClose()
	}
	return err
}

func (s *Server) buildListenerHandler(l config.Listener) (http.Handler, error) {
	handler := routing.NewEndpointHandler(s.router, l.Name)
	if len(l.Middlewares) > 0 {
		// Add middleware for this specific listener.
		middlewares, err := buildMiddlewareForListener(l.Middlewares)
		if err != nil {
			return nil, fmt.Errorf("listener '%s': %w", l.Name, err)
		}
		handler = routing.ChainMiddleware(handler, middlewares...)
	}
	handler = newRequestBodyLimiter(handler, l.MaxRequestBodySize)
	handler = newConnectionLimiter(handler, l.MaxConnections, l.Name)
	return metrics.InstrumentHandler(l.Name, l.Type, handler), nil
}

func (s *Server) startHTTPListener(l config.Listener) error {
	handler, err := s.buildListenerHandler(l)
	if err != nil {
		return err
	}

	srv := listener.NewHTTPServer(l.Address, handler)
	if readHeaderTimeout, err := time.ParseDuration(l.ReadHeaderTimeout); err == nil && readHeaderTimeout > 0 {
		srv.ReadHeaderTimeout = readHeaderTimeout
	}
	if writeTimeout, err := time.ParseDuration(l.WriteTimeout); err == nil && writeTimeout > 0 {
		srv.WriteTimeout = writeTimeout
	}
	if idleTimeout, err := time.ParseDuration(l.IdleTimeout); err == nil && idleTimeout > 0 {
		srv.IdleTimeout = idleTimeout
	}
	if !s.registerHTTPServer(srv) {
		return nil
	}
	log.Printf("HTTP listener '%s' started on '%s'\n", l.Name, l.Address)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server '%s' error: %w", l.Name, err)
	}
	return nil
}

func (s *Server) startHTTPSListener(l config.Listener) error {
	handler, err := s.buildListenerHandler(l)
	if err != nil {
		return err
	}

	srv, err := listener.NewHTTPSServer(
		l.Address,
		l.TLS.CertFile,
		l.TLS.KeyFile,
		l.TLS.CAFile,
		l.TLS.CRLFile,
		l.TLS.OCSPURL,
		handler,
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTPS server '%s': %w", l.Name, err)
	}
	if readHeaderTimeout, err := time.ParseDuration(l.ReadHeaderTimeout); err == nil && readHeaderTimeout > 0 {
		srv.ReadHeaderTimeout = readHeaderTimeout
	}
	if writeTimeout, err := time.ParseDuration(l.WriteTimeout); err == nil && writeTimeout > 0 {
		srv.WriteTimeout = writeTimeout
	}
	if idleTimeout, err := time.ParseDuration(l.IdleTimeout); err == nil && idleTimeout > 0 {
		srv.IdleTimeout = idleTimeout
	}
	if !s.registerHTTPServer(srv) {
		return nil
	}
	log.Printf("HTTPS listener '%s' started on %s\n", l.Name, l.Address)
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTPS server '%s' error: %w", l.Name, err)
	}
	return nil
}

func (s *Server) startDynamicHTTPSListener(l config.Listener) error {
	handler, err := s.buildListenerHandler(l)
	if err != nil {
		return err
	}
	srv, err := listener.NewDynamicHTTPSServer(l.Address, l.TLS.CertFile, l.TLS.KeyFile, l.TLS.KeyPassphrase, handler)
	if err != nil {
		return fmt.Errorf("failed to create dynamic HTTPS server '%s': %w", l.Name, err)
	}
	if readHeaderTimeout, err := time.ParseDuration(l.ReadHeaderTimeout); err == nil && readHeaderTimeout > 0 {
		srv.ReadHeaderTimeout = readHeaderTimeout
	}
	if writeTimeout, err := time.ParseDuration(l.WriteTimeout); err == nil && writeTimeout > 0 {
		srv.WriteTimeout = writeTimeout
	}
	if idleTimeout, err := time.ParseDuration(l.IdleTimeout); err == nil && idleTimeout > 0 {
		srv.IdleTimeout = idleTimeout
	}
	if !s.registerHTTPServer(srv) {
		return nil
	}
	log.Printf("Dynamic HTTPS listener '%s' started on %s\n", l.Name, l.Address)
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("dynamic HTTPS server '%s' error: %w", l.Name, err)
	}
	return nil
}

func (s *Server) startProxyListener(l config.Listener) error {
	srv := &http.Server{
		Addr:    l.Address,
		Handler: listener.NewProxyHandler(30 * time.Second),
	}
	if readHeaderTimeout, err := time.ParseDuration(l.ReadHeaderTimeout); err == nil && readHeaderTimeout > 0 {
		srv.ReadHeaderTimeout = readHeaderTimeout
	}
	if !s.registerHTTPServer(srv) {
		return nil
	}
	log.Printf("Proxy listener '%s' started on '%s'\n", l.Name, l.Address)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("proxy server '%s' error: %w", l.Name, err)
	}
	return nil
}

func (s *Server) startHTTP3Listener(l config.Listener) error {
	handler, err := s.buildListenerHandler(l)
	if err != nil {
		return err
	}

	srv, err := listener.NewHTTP3Server(
		l.Address,
		l.TLS.CertFile,
		l.TLS.KeyFile,
		l.TLS.CAFile,
		l.TLS.CRLFile,
		l.TLS.OCSPURL,
		handler,
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP/3 server '%s': %w", l.Name, err)
	}
	if !s.registerHTTP3Server(srv) {
		return nil
	}
	log.Printf("HTTP/3 listener '%s' started on %s\n", l.Name, l.Address)
	if err := srv.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			return fmt.Errorf("HTTP/3 server '%s' error: %w", l.Name, err)
		}
	}
	return nil
}

func (s *Server) startHTTPMultiListener(l config.Listener) error {
	handler, err := s.buildListenerHandler(l)
	if err != nil {
		return err
	}

	srv, err := listener.NewHTTPMultiServer(
		l.Address,
		l.TLS.CertFile,
		l.TLS.KeyFile,
		l.TLS.CAFile,
		l.TLS.CRLFile,
		l.TLS.OCSPURL,
		handler,
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP multi server '%s': %w", l.Name, err)
	}
	if !s.registerHTTPMultiServer(srv) {
		return nil
	}
	log.Printf("HTTP multi listener '%s' started on %s\n", l.Name, l.Address)
	if err := srv.ListenAndServeMulti(handler); err != nil {
		if err != httpmulti.ErrClosedListener {
			return fmt.Errorf("HTTP multi server '%s' error: %w", l.Name, err)
		}
	}
	return nil
}

func (s *Server) startTCPListener(l config.Listener) error {
	ln, err := listener.NewTCPListener(
		l.Address,
		l.TLS.CertFile,
		l.TLS.KeyFile,
		l.TLS.CAFile,
		l.TLS.CRLFile,
		l.TLS.OCSPURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create TCP listener '%s': %w", l.Name, err)
	}
	ln = newLimitedListener(ln, l.MaxConnections)
	if !s.registerTCPListener(ln) {
		return nil
	}
	log.Printf("TCP listener '%s' started on %s\n", l.Name, l.Address)
	if err := listener.HandleTCPListener(s.ctx, ln, l.Backends, l.Routes); err != nil && err != context.Canceled {
		return fmt.Errorf("TCP listener '%s' error: %w", l.Name, err)
	}
	return nil
}
