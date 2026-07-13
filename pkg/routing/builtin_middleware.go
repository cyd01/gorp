package routing

import (
	"log"
	"net"
	"net/http"
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

// CorrelationID adds a correlation ID when the request does not already have one.
func (bm *BuiltinMiddleware) CorrelationID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := r.Header["X-Correlation-Id"]; !ok {
				u := uuid.New().String()
				r.Header.Add("X-Correlation-Id", u)
				w.Header().Set("X-Correlation-Id", u)
				next.ServeHTTP(w, r)
			}
		})
	}
}

// Logging records request details and completion time.
func (bm *BuiltinMiddleware) Logging() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := r.Header.Get("X-Correlation-Id")
			start := time.Now()
			if len(c) > 0 {
				log.Printf("[%s] %s %s, %s", r.Method, r.RequestURI, r.RemoteAddr, c)
			} else {
				log.Printf("[%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)
			}
			next.ServeHTTP(w, r)
			if len(c) > 0 {
				log.Printf("[%s] %s completed in %v, %s", r.Method, r.RequestURI, time.Since(start), c)
			} else {
				log.Printf("[%s] %s completed in %v", r.Method, r.RequestURI, time.Since(start))
			}
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

// RateLimitSimple is a very simple rate-limiting middleware.
// This is a basic example; use a production-ready library in production.
func (bm *BuiltinMiddleware) RateLimitSimple(maxRequests int, window time.Duration) Middleware {
	type Bucket struct {
		count     int
		resetTime time.Time
	}
	buckets := make(map[string]*Bucket)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := strings.SplitN(r.RemoteAddr, ":", 2)[0]
			now := time.Now()

			bucket, exists := buckets[ip]
			if !exists {
				buckets[ip] = &Bucket{
					count:     1,
					resetTime: now.Add(window),
				}
			} else if now.After(bucket.resetTime) {
				buckets[ip] = &Bucket{
					count:     1,
					resetTime: now.Add(window),
				}
			} else {
				bucket.count++
				if bucket.count > maxRequests {
					http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AddPrefix adds a prefix to the request path.
func (bm *BuiltinMiddleware) AddPrefix(prefix string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(prefix, "/") {
				prefix = "/" + prefix
			}
			r.URL.Path = prefix + r.URL.Path
			next.ServeHTTP(w, r)
		})
	}
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
