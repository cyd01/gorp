package routing

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

const traceParentHeader = "traceparent"

// DistributedTracing propagates W3C Trace Context and creates a span for the proxy.
func (bm *BuiltinMiddleware) DistributedTracing() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID, _, flags, ok := parseTraceParent(r.Header.Get(traceParentHeader))
			if !ok {
				traceID = newTraceIdentifier(16)
				flags = "00"
			}

			spanID := newTraceIdentifier(8)
			traceParent := "00-" + traceID + "-" + spanID + "-" + flags
			r.Header.Set(traceParentHeader, traceParent)
			w.Header().Set(traceParentHeader, traceParent)
			next.ServeHTTP(w, r)
		})
	}
}

func parseTraceParent(value string) (traceID, parentSpanID, flags string, ok bool) {
	parts := strings.Split(value, "-")
	if len(parts) != 4 || parts[0] != "00" || len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return "", "", "", false
	}
	if !isLowerHex(parts[1]) || !isLowerHex(parts[2]) || !isLowerHex(parts[3]) {
		return "", "", "", false
	}
	if strings.Trim(parts[1], "0") == "" || strings.Trim(parts[2], "0") == "" {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[3], true
}

func isLowerHex(value string) bool {
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func newTraceIdentifier(size int) string {
	identifier := make([]byte, size)
	if _, err := rand.Read(identifier); err != nil {
		panic("crypto/rand unavailable")
	}
	return hex.EncodeToString(identifier)
}
