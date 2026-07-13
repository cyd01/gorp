package routing

import (
	"net/http"
)

// Middleware wraps an HTTP handler.
type Middleware func(http.Handler) http.Handler

// ChainMiddleware combines multiple middleware in order.
func ChainMiddleware(handler http.Handler, middlewares ...Middleware) http.Handler {
	// Apply middleware in reverse order so the first item is executed first.
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// MiddlewareRegistry stores the available middleware.
type MiddlewareRegistry struct {
	middleware map[string]Middleware
}

// NewMiddlewareRegistry creates a middleware registry.
func NewMiddlewareRegistry() *MiddlewareRegistry {
	return &MiddlewareRegistry{
		middleware: make(map[string]Middleware),
	}
}

// Register registers middleware under a name.
func (mr *MiddlewareRegistry) Register(name string, mw Middleware) {
	mr.middleware[name] = mw
}

// Get retrieves middleware by name.
func (mr *MiddlewareRegistry) Get(name string) (Middleware, bool) {
	mw, ok := mr.middleware[name]
	return mw, ok
}

// GetAll retrieves multiple middleware by name.
func (mr *MiddlewareRegistry) GetAll(names []string) []Middleware {
	middlewares := make([]Middleware, 0, len(names))
	for _, name := range names {
		if mw, ok := mr.Get(name); ok {
			middlewares = append(middlewares, mw)
		}
	}
	return middlewares
}
