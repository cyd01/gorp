package routing

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BuiltinMiddleware contains the available built-in middleware.
type BuiltinMiddleware struct{}

// NewBuiltinMiddleware creates a built-in middleware instance.
func NewBuiltinMiddleware() *BuiltinMiddleware {
	return &BuiltinMiddleware{}
}

// Delay adds a delay before processing the request.
func (bm *BuiltinMiddleware) Delay(duration time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(duration)
			next.ServeHTTP(w, r)
		})
	}
}

// Prefer adds a middleware that does nothing unless the 'Prefer' header is present in the request.
func (bm *BuiltinMiddleware) Prefer() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			preferHeader := r.Header.Values("Prefer")
			for i := range preferHeader {
				if strings.EqualFold(preferHeader[i], "break") {
					fmt.Printf("break\n")
					return
				}
				if strings.HasPrefix(strings.ToLower(preferHeader[i]), "delay=") {
					delayValue := strings.SplitN(preferHeader[i], "=", 2)[1]
					delayDuration, err := time.ParseDuration(delayValue)
					if err == nil {
						time.Sleep(delayDuration)
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CorrelationID adds a correlation ID when the request does not already have one.
func (bm *BuiltinMiddleware) CorrelationID(uuidtype string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := r.Header.Get("X-Correlation-Id")
			if correlationID == "" {
				switch strings.ToLower(uuidtype) {
				case "uuidv6":
					uid, err := uuid.NewV6()
					if err == nil {
						correlationID = uid.String()
					} else {
						correlationID = uuid.New().String()
					}
				case "uuidv7":
					uid, err := uuid.NewV7()
					if err == nil {
						correlationID = uid.String()
					} else {
						correlationID = uuid.New().String()
					}
				default:
					correlationID = uuid.New().String()
				}
			}
			r.Header.Set("X-Correlation-Id", correlationID)
			w.Header().Set("X-Correlation-Id", correlationID)
			next.ServeHTTP(w, r)
		})
	}
}

// CORS configures Cross-Origin Resource Sharing headers.
func (bm *BuiltinMiddleware) CORS() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Configure CORS headers.
			w.Header().Set("Access-Control-Allow-Origin", "*") // This can be restricted to specific domains.
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			// Handle preflight (OPTIONS) requests.
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AddPrefix adds a prefix to the request path.
func (bm *BuiltinMiddleware) AddPrefix(prefix string) Middleware {
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = prefix + r.URL.Path
			next.ServeHTTP(w, r)
		})
	}
}

// RewritePath replaces all matches of pattern in the request URL path.
func (bm *BuiltinMiddleware) RewritePath(pattern, replacement string) (Middleware, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile path rewrite regex: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = re.ReplaceAllString(r.URL.Path, replacement)
			r.URL.RawPath = ""
			next.ServeHTTP(w, r)
		})
	}, nil
}

// IPWhitelist checks whether the source IP is in the allowed CIDR list.
func (bm *BuiltinMiddleware) IPWhitelist(allowedCIDRs []string) Middleware {
	var networks []*net.IPNet
	for _, cidr := range allowedCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			log.Printf("Warning: invalid CIDR %s", cidr)
			continue
		}
		networks = append(networks, ipNet)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			ip := net.ParseIP(host)
			if ip == nil {
				ip = net.ParseIP(r.RemoteAddr)
			}

			allowed := false
			if ip != nil {
				for _, network := range networks {
					if network.Contains(ip) {
						allowed = true
						break
					}
				}
			}

			if !allowed {
				log.Printf("'%s' not in allowed CIDRs\n", r.RemoteAddr)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
