package listener

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/helper"
	"github.com/cyd01/gorp/pkg/routing"
)

func StartTCP(address, cert, key, ca, crl, ocspURL string, backends []config.Backend) error {
	if len(backends) == 0 {
		log.Println("Warning: no backends configured for TCP listener")
	}

	listener, err := NewTCPListener(address, cert, key, ca, crl, ocspURL)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("TCP listener started on %s\n", address)

	if err := HandleTCPListener(context.Background(), listener, backends, nil); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func NewTCPListener(address, cert, key, ca, crl, ocspURL string) (net.Listener, error) {
	var listener net.Listener
	var err error

	// Check if TLS is configured
	if (cert != "") && (key != "") {
		tlsConfig, err := helper.BuildTLSConfigForDownstream(cert, key, ca, crl, ocspURL)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}
		rawListener, err := net.Listen("tcp", address)
		if err != nil {
			return nil, err
		}
		listener = &tlsListener{Listener: rawListener, config: tlsConfig}
	} else {
		listener, err = net.Listen("tcp", address)
	}

	return listener, err
}

type tlsListener struct {
	net.Listener
	config *tls.Config
}

func HandleTCPListener(ctx context.Context, listener net.Listener, backends []config.Backend, routeSets ...[]config.TCPRoute) error {
	var routes []config.TCPRoute
	if len(routeSets) > 0 {
		routes = routeSets[0]
	}
	if len(backends) == 0 {
		if len(routes) == 0 {
			return fmt.Errorf("no backends configured for TCP listener")
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Set a read deadline to allow context checking
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		clientConn, err := listener.Accept()
		if err != nil {
			// Check if it's a timeout or context canceled
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			// If context is done, exit gracefully
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				log.Printf("Error accepting connection: %v\n", err)
				continue
			}
		}

		go handleTCPConnection(clientConn, backends, routes, listener)
	}
}

func handleTCPConnection(clientConn net.Conn, backends []config.Backend, routes []config.TCPRoute, listener net.Listener) {
	defer func() {
		_ = clientConn.Close()
	}()

	if tlsListener, ok := listener.(*tlsListener); ok {
		if err := clientConn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			log.Printf("Error setting TLS handshake deadline: %v\n", err)
		}
		hello, reader, err := peekClientHello(clientConn)
		if err != nil {
			log.Printf("Error reading TLS ClientHello: %v\n", err)
			return
		}
		_ = clientConn.SetReadDeadline(time.Time{})
		selectedRoute := selectTCPRoute(hello.ServerName, routes)
		backends = selectTCPBackends(hello.ServerName, backends, routes)
		if len(backends) == 0 {
			log.Printf("No backend configured for SNI %q\n", hello.ServerName)
			return
		}
		serverTLSConfig := tlsListener.config
		if selectedRoute != nil && selectedRoute.TLS != nil && selectedRoute.TLS.CertFile != "" && selectedRoute.TLS.KeyFile != "" {
			serverTLSConfig, err = helper.BuildTLSConfigForDownstream(selectedRoute.TLS.CertFile, selectedRoute.TLS.KeyFile, selectedRoute.TLS.CAFile, selectedRoute.TLS.CRLFile, selectedRoute.TLS.OCSPURL)
			if err != nil {
				log.Printf("Error building TLS config for route %q: %v\n", selectedRoute.Name, err)
				return
			}
		}
		clientConn = tls.Server(&bufferedConn{Conn: clientConn, reader: reader}, serverTLSConfig)
		if err := clientConn.(*tls.Conn).Handshake(); err != nil {
			log.Printf("Error completing TLS handshake: %v\n", err)
			return
		}
	}

	if len(backends) == 0 {
		log.Println("No available backends")
		return
	}

	// Select a random backend
	backend := backends[rand.Intn(len(backends))]

	// Parse backend URL (expected format: tcp://host:port)
	backendAddr, _ := strings.CutPrefix(backend.URL, "tcp://")
	if backendAddr == "" {
		log.Printf("Backend %s has empty URL\n", backend.Name)
		return
	}

	// Connect to backend
	var backendConn net.Conn
	var err error
	if backend.ForceTLS {
		tlsConfig, _ := helper.BuildTLSConfigForUpstream(backend.TLS.Insecure, backend.TLS.ServerName, backend.TLS.CAFile, backend.TLS.CertFile, backend.TLS.KeyFile)
		backendConn, err = tls.Dial("tcp", backendAddr, tlsConfig)
		if err != nil {
			log.Printf("Error connecting to backend %s with TLS: %v\n", backend.Name, err)
			return
		}
	} else {
		timeout := 30 * time.Second
		if len(backend.Timeout) > 0 {
			if d, err := time.ParseDuration(backend.Timeout); err == nil {
				timeout = d
			}
		}
		backendConn, err = net.DialTimeout("tcp", backendAddr, timeout)
		if err != nil {
			log.Printf("Error connecting to backend %s: %v\n", backend.Name, err)
			return
		}
	}
	defer backendConn.Close()

	// Bidirectional tunneling
	/*
		go io.Copy(backendConn, clientConn)
		io.Copy(clientConn, backendConn)
	*/

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(clientConn, backendConn)
		closeWrite(clientConn)
		wg.Done()
	}()
	go func() {
		io.Copy(backendConn, clientConn)
		if backend.ForceTLS {
			backendConn.(*tls.Conn).CloseWrite()
		} else {
			backendConn.(*net.TCPConn).CloseWrite()
		}
		wg.Done()
	}()

	wg.Wait()
}

func closeWrite(conn net.Conn) {
	if writer, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = writer.CloseWrite()
	}
}

type bufferedConn struct {
	net.Conn
	reader io.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func selectTCPRoute(serverName string, routes []config.TCPRoute) *config.TCPRoute {
	for _, route := range routes {
		for _, host := range route.Hosts {
			if routing.MatchWildcard(strings.ToLower(host), strings.ToLower(serverName)) {
				return &route
			}
		}
	}
	return nil
}

func selectTCPBackends(serverName string, fallback []config.Backend, routes []config.TCPRoute) []config.Backend {
	for _, route := range routes {
		for _, host := range route.Hosts {
			if routing.MatchWildcard(strings.ToLower(host), strings.ToLower(serverName)) {
				return route.Backends
			}
		}
	}
	return fallback
}
