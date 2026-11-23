package routing

import (
	"net/http"
	"strings"
)

// MethodAllowList permits only the configured HTTP methods.
func (bm *BuiltinMiddleware) MethodAllowList(methods []string) Middleware {
	allowed := uniqueMethods(methods)
	allowHeader := strings.Join(allowed, ", ")
	allowedMethods := makeMethodSet(allowed)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowedMethods[r.Method] {
				w.Header().Set("Allow", allowHeader)
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MethodDenyList rejects the configured HTTP methods.
func (bm *BuiltinMiddleware) MethodDenyList(methods []string) Middleware {
	denied := makeMethodSet(methods)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if denied[r.Method] {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func makeMethodSet(methods []string) map[string]bool {
	set := make(map[string]bool, len(methods))
	for _, method := range methods {
		set[method] = true
	}
	return set
}

func uniqueMethods(methods []string) []string {
	seen := make(map[string]bool, len(methods))
	unique := make([]string, 0, len(methods))
	for _, method := range methods {
		if !seen[method] {
			seen[method] = true
			unique = append(unique, method)
		}
	}
	return unique
}
