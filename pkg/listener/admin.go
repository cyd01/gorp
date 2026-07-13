package listener

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cyd01/gorp/pkg/backend"
	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/helper"
	"github.com/cyd01/gorp/pkg/metrics"
)

// ShutdownFunc is the signature of a shutdown function.
type ShutdownFunc func(ctx context.Context)

func StartAdmin(address, cert, key, ca, crl, ocspURL string, cfg *config.Config, shutdownFn ShutdownFunc, minVersion ...string) error {
	if address == "" {
		return fmt.Errorf("admin address not configured")
	}

	srv, err := NewAdminServer(address, cert, key, ca, crl, ocspURL, cfg, shutdownFn, minVersion...)
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

func NewAdminServer(address, cert, key, ca, crl, ocspURL string, cfg *config.Config, shutdownFn ShutdownFunc, minVersion ...string) (*http.Server, error) {
	return NewAdminServerWithBackends(address, cert, key, ca, crl, ocspURL, cfg, shutdownFn, nil, minVersion...)
}

func NewAdminServerWithBackends(address, cert, key, ca, crl, ocspURL string, cfg *config.Config, shutdownFn ShutdownFunc, backends []*backend.Backend, minVersion ...string) (*http.Server, error) {
	return NewAdminServerWithBackendsAndKeyPassphrase(address, cert, key, "", ca, crl, ocspURL, cfg, shutdownFn, backends, minVersion...)
}

func NewAdminServerWithBackendsAndKeyPassphrase(address, cert, key, keyPassphrase, ca, crl, ocspURL string, cfg *config.Config, shutdownFn ShutdownFunc, backends []*backend.Backend, minVersion ...string) (*http.Server, error) {
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

	// Code endpoint
	mux.Handle("/code/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		codest, _ := strings.CutPrefix(r.URL.Path, "/code/")
		code, err := strconv.Atoi(codest)
		if err != nil {
			code = 200
		}
		headers := r.Header.Get("X-Headers")
		if len(headers) > 0 {
			hs := strings.Split(headers, ";")
			for _, h := range hs {
				v := strings.SplitN(h, ":", 2)
				w.Header().Add(strings.TrimSpace(v[0]), strings.TrimSpace(v[1]))
			}
		}
		if (code >= 300) && (code < 400) {
			loc := r.Header.Get("X-Location")
			if len(loc) > 0 {
				w.Header().Add("Location", strings.TrimSpace(loc))
			}
		}
		w.WriteHeader(code)
		if (code != 204) && (code != 304) {
			body := r.Header.Get("X-Body")
			if len(body) > 0 {
				w.Write([]byte(body))
			} else {
				w.Write([]byte(http.StatusText(code)))
			}
		}
	}))

	// Ready check endpoint
	mux.HandleFunc("/ready", helper.Ready.Handler)

	// Backend status endpoint
	mux.HandleFunc("/backends", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type backendStatus struct {
			Name        string `json:"name"`
			Destination string `json:"destination"`
			State       string `json:"state"`
		}
		statuses := make([]backendStatus, 0, len(backends))
		for _, be := range backends {
			if be == nil || be.URL == nil {
				continue
			}
			statuses = append(statuses, backendStatus{
				Name:        be.Name,
				Destination: be.URL.String(),
				State:       be.State(),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(statuses); err != nil {
			log.Printf("Error while encoding backend status: %v\n", err)
			http.Error(w, "Error while encoding backend status", http.StatusInternalServerError)
		}
	})

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
		configuredMinVersion := ""
		if len(minVersion) > 0 {
			configuredMinVersion = minVersion[0]
		}
		tlsConfig, err = helper.BuildTLSConfigForDownstreamWithMinVersion(cert, key, ca, crl, ocspURL, configuredMinVersion, keyPassphrase)
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
