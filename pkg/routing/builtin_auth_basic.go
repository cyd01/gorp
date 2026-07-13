package routing

import (
	"fmt"
	"net/http"
)

// BasicAuth requires a username and password.
func (bm *BuiltinMiddleware) BasicAuth(realm string, users map[string]string) Middleware {
	if realm == "" {
		realm = "Restricted"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if ok {
				if expected, found := users[username]; found && password == expected {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}
