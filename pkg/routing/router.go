package routing

import (
	"net/http"
)

type Router struct {
	Routes    []*Route
	Directory string
}

func New(routes []*Route, directory string) *Router {
	return &Router{
		Routes:    routes,
		Directory: directory,
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.ServeHTTPForEndpoint(w, req, "")
}

func (r *Router) ServeHTTPForEndpoint(w http.ResponseWriter, req *http.Request, endpoint string) {
	for _, route := range r.Routes {
		if route.MatchEndpoint(endpoint) && route.Match(req) {
			route.ServeHTTP(w, req)
			return
		}
	}
	if len(r.Directory) > 0 {
		http.ServeFile(w, req, r.Directory+req.URL.Path)
		return
	}
	http.NotFound(w, req)
}

type EndpointHandler struct {
	router   *Router
	endpoint string
}

func NewEndpointHandler(router *Router, endpoint string) http.Handler {
	return &EndpointHandler{router: router, endpoint: endpoint}
}

func (h *EndpointHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.router.ServeHTTPForEndpoint(w, req, h.endpoint)
}
