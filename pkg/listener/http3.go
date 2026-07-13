package listener

import (
	"fmt"
	"log"
	"net/http"

	"github.com/cyd01/gorp/pkg/helper"

	"github.com/quic-go/quic-go/http3"
)

func StartHTTP3(address, cert, key, ca, crl, ocspURL string, handler http.Handler) error {
	srv, err := NewHTTP3Server(address, cert, key, ca, crl, ocspURL, handler)
	if err != nil {
		return err
	}
	log.Printf("HTTP/3 listener started on %s\n", address)
	return srv.ListenAndServe()
}

func NewHTTP3Server(address, cert, key, ca, crl, ocspURL string, handler http.Handler) (*http3.Server, error) {
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	tlsConfig, err := helper.BuildTLSConfigForDownstream(cert, key, ca, crl, ocspURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	srv := &http3.Server{
		Addr:      address,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	return srv, nil
}
