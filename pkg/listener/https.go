package listener

import (
	"fmt"
	"log"
	"net/http"

	"github.com/cyd01/gorp/pkg/helper"
)

func StartHTTPS(address, cert, key, ca, crl, ocspURL string, handler http.Handler) error {
	srv, err := NewHTTPSServer(address, cert, key, ca, crl, ocspURL, handler)
	if err != nil {
		return err
	}
	log.Printf("HTTPS listener started on %s\n", address)
	return srv.ListenAndServeTLS(cert, key)
}

func NewHTTPSServer(address, cert, key, ca, crl, ocspURL string, handler http.Handler) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	tlsConfig, err := helper.BuildTLSConfigForDownstream(cert, key, ca, crl, ocspURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	srv := &http.Server{
		Addr:      address,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	return srv, nil
}

func NewDynamicHTTPSServer(address, caCert, caKey, caKeyPassphrase string, handler http.Handler) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	tlsConfig, err := helper.BuildDynamicTLSConfig(caCert, caKey, caKeyPassphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to build dynamic TLS config: %w", err)
	}

	return &http.Server{Addr: address, Handler: mux, TLSConfig: tlsConfig}, nil
}
