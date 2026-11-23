package routing

import (
	"fmt"
	"net/http"
	"strings"
)

// TokenAuth validates a Bearer token or equivalent value in an HTTP header.
func (bm *BuiltinMiddleware) TokenAuth(realm string, tokens []string, header, prefix string) Middleware {
	if realm == "" {
		realm = "Restricted"
	}
	if header == "" {
		header = "Authorization"
	}
	if prefix == "" {
		prefix = "Bearer "
	}

	allowed := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		allowed[token] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			value := r.Header.Get(header)
			if strings.HasPrefix(value, prefix) {
				candidate := strings.TrimPrefix(value, prefix)
				if _, ok := allowed[candidate]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, realm))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}
