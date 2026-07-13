package routing

import (
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

type rateLimitBucket struct {
	count     int
	resetTime time.Time
}

type rateLimiter struct {
	mu              sync.Mutex
	buckets         map[string]*rateLimitBucket
	window          time.Duration
	maxRequests     int
	nextCleanup     time.Time
	cleanupInterval time.Duration
}

func newRateLimiter(maxRequests int, window time.Duration) *rateLimiter {
	cleanupInterval := window
	if cleanupInterval <= 0 || cleanupInterval > time.Minute {
		cleanupInterval = time.Minute
	}
	return &rateLimiter{
		buckets:         make(map[string]*rateLimitBucket),
		window:          window,
		maxRequests:     maxRequests,
		cleanupInterval: cleanupInterval,
	}
}

func (limiter *rateLimiter) allow(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if limiter.nextCleanup.IsZero() || !now.Before(limiter.nextCleanup) {
		for bucketKey, bucket := range limiter.buckets {
			if !now.Before(bucket.resetTime) {
				delete(limiter.buckets, bucketKey)
			}
		}
		limiter.nextCleanup = now.Add(limiter.cleanupInterval)
	}

	bucket, exists := limiter.buckets[key]
	if !exists || !now.Before(bucket.resetTime) {
		limiter.buckets[key] = &rateLimitBucket{
			count:     1,
			resetTime: now.Add(limiter.window),
		}
		return true
	}

	bucket.count++
	return bucket.count <= limiter.maxRequests
}

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
	limiter := newRateLimiter(maxRequests, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(patterns) > 0 && !matchesAnyPath(patterns, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r.RemoteAddr)
			now := time.Now()
			if !limiter.allow(ip, now) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
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
