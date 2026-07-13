package listener

import (
	"fmt"
	"log"
	"net/http"

	"github.com/cyd01/gorp/pkg/helper"
)

func StartHTTPS(address, cert, key, ca, crl, ocspURL string, handler http.Handler, minVersion ...string) error {
	srv, err := NewHTTPSServer(address, cert, key, ca, crl, ocspURL, handler, minVersion...)
	if err != nil {
		return err
	}
	log.Printf("HTTPS listener started on %s\n", address)
	return srv.ListenAndServeTLS(cert, key)
}

func NewHTTPSServer(address, cert, key, ca, crl, ocspURL string, handler http.Handler, minVersion ...string) (*http.Server, error) {
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

	srv := &http.Server{
		Addr:      address,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	return srv, nil
}

func NewDynamicHTTPSServer(address, caCert, caKey, caKeyPassphrase string, handler http.Handler, minVersion ...string) (*http.Server, error) {
	return NewDynamicHTTPSServerWithClientCA(address, caCert, caKey, caKeyPassphrase, "", handler, minVersion...)
}

func NewDynamicHTTPSServerWithClientCA(address, caCert, caKey, caKeyPassphrase, clientCA string, handler http.Handler, minVersion ...string) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.Handle("/", withForwardedClientCertificate(handler))

	tlsConfig, err := helper.BuildDynamicTLSConfigWithClientCA(caCert, caKey, caKeyPassphrase, clientCA, minVersion...)
	if err != nil {
		return nil, fmt.Errorf("failed to build dynamic TLS config: %w", err)
	}

	return &http.Server{Addr: address, Handler: mux, TLSConfig: tlsConfig}, nil
}
