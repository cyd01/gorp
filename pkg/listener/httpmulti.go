package listener

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cyd01/gorp/pkg/helper"
	"github.com/cyd01/gorp/pkg/httpmulti"
)

func StartHTTPMulti(address, cert, key, ca, crl, ocspURL string, handler http.Handler, minVersion ...string) error {
	srv, err := NewHTTPMultiServer(address, cert, key, ca, crl, ocspURL, handler, minVersion...)
	if err != nil {
		return err
	}
	log.Printf("HTTP multi listener started on %s\n", address)
	return srv.ListenAndServeMulti(handler)
}

func NewHTTPMultiServer(address, cert, key, ca, crl, ocspURL string, handler http.Handler, minVersion ...string) (*httpmulti.Server, error) {
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	configuredMinVersion := ""
	if len(minVersion) > 0 {
		configuredMinVersion = minVersion[0]
	}
	tlsConfig, err := helper.BuildTLSConfigForDownstreamWithMinVersion(cert, key, ca, crl, ocspURL, configuredMinVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	srv := &httpmulti.Server{
		Addr: address,
		Server: http.Server{
			Addr:      address,
			Handler:   mux,
			TLSConfig: tlsConfig,
		},
		Proto:       "tcp",
		ReadTimeout: time.Duration(500) * time.Millisecond,
	}

	srv.SetHTTP1(true)
	srv.SetHTTP2(true)
	srv.SetUnencryptedHTTP2(true)
	srv.SetCACertFile(ca)
	srv.SetCRL(crl)
	//srv.SetLogger(log.Default())

	return srv, nil
}
