package routing

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DigestAuth applies a simplified RFC 7616 Digest challenge.
func (bm *BuiltinMiddleware) DigestAuth(realm string, users map[string]string, timeout time.Duration) Middleware {
	if realm == "" {
		realm = "Restricted"
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	type nonceEntry struct {
		expiry time.Time
	}
	nonces := make(map[string]nonceEntry)
	var nonceMu sync.Mutex

	generateChallenge := func() string {
		nonce := randomToken(24)
		opaque := randomToken(24)
		nonceMu.Lock()
		nonces[nonce] = nonceEntry{expiry: time.Now().Add(timeout)}
		nonceMu.Unlock()
		return fmt.Sprintf(`Digest realm="%s", nonce="%s", opaque="%s", algorithm=MD5, qop="auth"`, realm, nonce, opaque)
	}

	isValidNonce := func(value string) bool {
		nonceMu.Lock()
		defer nonceMu.Unlock()
		entry, ok := nonces[value]
		if !ok {
			return false
		}
		if time.Now().After(entry.expiry) {
			delete(nonces, value)
			return false
		}
		return true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Digest ") {
				w.Header().Set("WWW-Authenticate", generateChallenge())
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			params := parseDigestParams(strings.TrimPrefix(authHeader, "Digest "))
			username := params["username"]
			realmValue := params["realm"]
			nonce := params["nonce"]
			uri := params["uri"]
			response := params["response"]
			qop := params["qop"]
			nc := params["nc"]
			cnonce := params["cnonce"]

			if username == "" || realmValue != realm || nonce == "" || uri == "" || response == "" || qop != "auth" || nc == "" || cnonce == "" {
				w.Header().Set("WWW-Authenticate", generateChallenge())
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !isValidNonce(nonce) {
				w.Header().Set("WWW-Authenticate", generateChallenge())
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			password, ok := users[username]
			if !ok {
				w.Header().Set("WWW-Authenticate", generateChallenge())
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			hA1 := md5Hash(fmt.Sprintf("%s:%s:%s", username, realm, password))
			hA2 := md5Hash(fmt.Sprintf("%s:%s", r.Method, uri))
			expected := md5Hash(fmt.Sprintf("%s:%s:%s:%s:%s:%s", hA1, nonce, nc, cnonce, qop, hA2))

			if expected != response {
				w.Header().Set("WWW-Authenticate", generateChallenge())
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func md5Hash(value string) string {
	h := md5.Sum([]byte(value))
	return fmt.Sprintf("%x", h)
}

func randomToken(size int) string {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func parseDigestParams(header string) map[string]string {
	values := make(map[string]string)
	for len(header) > 0 {
		header = strings.TrimLeft(header, " ")
		keyEnd := strings.IndexAny(header, "=,")
		if keyEnd == -1 {
			break
		}
		key := strings.TrimSpace(header[:keyEnd])
		if len(header) <= keyEnd || header[keyEnd] != '=' {
			header = strings.TrimLeft(header[keyEnd+1:], " ,")
			continue
		}
		header = strings.TrimLeft(header[keyEnd+1:], " ")
		value := ""
		if strings.HasPrefix(header, "\"") {
			header = header[1:]
			end := strings.Index(header, "\"")
			if end == -1 {
				break
			}
			value = header[:end]
			header = strings.TrimLeft(header[end+1:], " ,")
		} else {
			end := strings.Index(header, ",")
			if end == -1 {
				value = strings.TrimSpace(header)
				header = ""
			} else {
				value = strings.TrimSpace(header[:end])
				header = strings.TrimLeft(header[end+1:], " ,")
			}
		}
		values[key] = value
	}
	return values
}
