package routing

import (
	"net/http"
)

// AddRequestHeaders adds headers to the incoming request.
func (bm *BuiltinMiddleware) AddRequestHeaders(headers map[string]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, value := range headers {
				r.Header.Add(key, value)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SetRequestHeader replaces headers on the incoming request.
func (bm *BuiltinMiddleware) SetRequestHeader(headers map[string]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, value := range headers {
				r.Header.Set(key, value)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ModifyRequestHeaders replaces headers on the incoming request.
func (bm *BuiltinMiddleware) ModifyRequestHeaders(headers map[string]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, value := range headers {
				r.Header.Set(key, value)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RemoveRequestHeaders removes headers from the incoming request.
func (bm *BuiltinMiddleware) RemoveRequestHeaders(headerNames []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, name := range headerNames {
				delete(r.Header, name)
			}
			next.ServeHTTP(w, r)
		})
	}
}
