package routing

import (
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

// RateLimitSimple is a very simple rate-limiting middleware.
// This is a basic example; use a production-ready library in production.
func (bm *BuiltinMiddleware) RateLimitSimple(maxRequests int, window time.Duration) Middleware {
	return bm.rateLimit(maxRequests, window, nil)
}

// RateLimitExtensions limits requests whose path matches one of the supplied
// wildcard patterns. Requests for other paths pass through without a quota.
func (bm *BuiltinMiddleware) RateLimitExtensions(maxRequests int, window time.Duration, extensions []string) Middleware {
	return bm.rateLimit(maxRequests, window, extensions)
}

func (bm *BuiltinMiddleware) rateLimit(maxRequests int, window time.Duration, patterns []string) Middleware {
	type Bucket struct {
		count     int
		resetTime time.Time
	}
	buckets := make(map[string]*Bucket)
	var mutex sync.Mutex
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(patterns) > 0 && !matchesAnyPath(patterns, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r.RemoteAddr)
			now := time.Now()

			mutex.Lock()
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
					mutex.Unlock()
					http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
					return
				}
			}
			mutex.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

func matchesAnyPath(patterns []string, requestPath string) bool {
	for _, pattern := range patterns {
		if MatchWildcard(pattern, requestPath) || MatchWildcard(pattern, path.Base(requestPath)) {
			return true
		}
	}
	return false
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(remoteAddr)
}
