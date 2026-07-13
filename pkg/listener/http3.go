package listener

import (
	"fmt"
	"log"
	"net/http"

	"github.com/cyd01/gorp/pkg/helper"

	"github.com/quic-go/quic-go/http3"
)

func StartHTTP3(address, cert, key, ca, crl, ocspURL string, handler http.Handler, minVersion ...string) error {
	srv, err := NewHTTP3Server(address, cert, key, ca, crl, ocspURL, handler, minVersion...)
	if err != nil {
		return err
	}
	log.Printf("HTTP/3 listener started on %s\n", address)
	return srv.ListenAndServe()
}

func NewHTTP3Server(address, cert, key, ca, crl, ocspURL string, handler http.Handler, minVersion ...string) (*http3.Server, error) {
	mux := http.NewServeMux()
	mux.Handle("/", withForwardedClientCertificate(handler))

	configuredMinVersion := ""
	if len(minVersion) > 0 {
		configuredMinVersion = minVersion[0]
	}
	tlsConfig, err := helper.BuildTLSConfigForDownstreamWithMinVersion(cert, key, ca, crl, ocspURL, configuredMinVersion)
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
