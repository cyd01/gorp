package listener

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/helper"
	"github.com/cyd01/gorp/pkg/metrics"
)

// ShutdownFunc is the signature of a shutdown function.
type ShutdownFunc func(ctx context.Context)

func StartAdmin(address, cert, key, ca, crl, ocspURL string, cfg *config.Config, shutdownFn ShutdownFunc) error {
	if address == "" {
		return fmt.Errorf("admin address not configured")
	}

	srv, err := NewAdminServer(address, cert, key, ca, crl, ocspURL, cfg, shutdownFn)
	if err != nil {
		return err
	}

	log.Printf("Admin listener started on %s\n", address)

	// Start with TLS if certificates are provided
	if cert != "" && key != "" {
		return srv.ListenAndServeTLS(cert, key)
	}
	return srv.ListenAndServe()
}

func NewAdminServer(address, cert, key, ca, crl, ocspURL string, cfg *config.Config, shutdownFn ShutdownFunc) (*http.Server, error) {
	if address == "" {
		return nil, fmt.Errorf("admin address not configured")
	}

	mux := http.NewServeMux()

	// echo endpoint
	mux.Handle("/echo/", http.StripPrefix("/echo", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		echoHandler(w, r)
	})))

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	})

	// Ready check endpoint
	mux.HandleFunc("/ready", helper.Ready.Handler)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", metrics.Handler())

	// Stop endpoint - triggers graceful shutdown
	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		// Respond before shutting down
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Shutting down...\n"))

		// Give the response time to be sent before shutting down
		go func() {
			time.Sleep(100 * time.Millisecond)
			if shutdownFn != nil {
				log.Printf("Shutting down...\n")
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				shutdownFn(ctx)
			}
		}()
	})

	// Serve the configuration.
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(cfg)
		if err != nil {
			http.Error(w, "Error while encofding JSON", http.StatusInternalServerError)
			return
		}
	})

	// Build TLS config if certificates are provided
	var tlsConfig *tls.Config
	if cert != "" && key != "" {
		var err error
		tlsConfig, err = helper.BuildTLSConfigForDownstream(cert, key, ca, crl, ocspURL)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}
	}

	srv := &http.Server{
		Addr:      address,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	return srv, nil
}
