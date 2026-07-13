package listener

import (
	"log"
	"net/http"
)

func StartHTTP(address string, handler http.Handler) error {
	srv := NewHTTPServer(address, handler)
	log.Printf("HTTP listener started on %s\n", address)
	return srv.ListenAndServe()
}

func NewHTTPServer(address string, handler http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	return &http.Server{
		Addr:    address,
		Handler: mux,
	}
}
