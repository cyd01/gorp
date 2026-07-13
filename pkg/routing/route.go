package routing

import (
	"net/http"
	"strings"

	"github.com/cyd01/gorp/pkg/selector"
)

type Route struct {
	Name        string
	Prefix      string
	StripPrefix bool
	Hosts       []string
	Endpoints   []string
	Selector    selector.Selector
	Middleware  []Middleware
}

func (r *Route) MatchEndpoint(endpoint string) bool {
	if len(r.Endpoints) == 0 {
		return true
	}
	for _, configuredEndpoint := range r.Endpoints {
		if configuredEndpoint == endpoint {
			return true
		}
	}
	return false
}

func (r *Route) Match(req *http.Request) bool {
	if r.Prefix != "" && strings.HasPrefix(req.URL.Path, r.Prefix) {
		return true
	}
	if req.Method == http.MethodConnect && r.Prefix == "/" {
		return true
	}
	for _, host := range r.Hosts {
		if MatchWildcard(host, req.Host) {
			return true
		}
	}
	return false
}

func (r *Route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	b, err := r.Selector.Select(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if b == nil {
		http.Error(w, "no backend", http.StatusServiceUnavailable)
		return
	}
	if r.StripPrefix &&
		r.Prefix != "" {
		req.URL.Path = strings.TrimPrefix(req.URL.Path, r.Prefix)
		if !strings.HasPrefix(req.URL.Path, "/") {
			req.URL.Path = "/" + req.URL.Path
		}
	}

	// Wrap the backend with the route middleware.
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodConnect {
			if err := b.Connect(w, req); err != nil {
				http.Error(w, "backend tunnel unavailable", http.StatusBadGateway)
			}
			return
		}
		b.Proxy.ServeHTTP(w, req)
	})
	if len(r.Middleware) > 0 {
		handler = ChainMiddleware(handler, r.Middleware...)
	}
	handler.ServeHTTP(w, req)
}
