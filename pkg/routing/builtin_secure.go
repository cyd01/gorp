package routing

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (bm *BuiltinMiddleware) Secure(path, key, redirect string, ttl time.Duration) Middleware {
	secret := []byte(key + time.Now().String())

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if r.URL.Path == path { // Generate the access cookie.
				token, err := generateCookie(r.UserAgent(), 15*time.Second, secret)
				if err != nil {
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				var secure bool
				if r.TLS != nil {
					secure = true
				}
				http.SetCookie(w, &http.Cookie{
					Name:     "access",
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					Secure:   secure,
					SameSite: http.SameSiteStrictMode,
				})
				if redirect != "" {
					w.Header().Set("Cache-Control", "no-cache,no-store,max-age=0,s-max-age=0,must-revalidate")
					http.Redirect(w, r, redirect, http.StatusFound)
					return
				} else {
					w.Header().Add("Content-Type", "text/plain")
					w.Header().Set("Cache-Control", "no-cache,no-store,max-age=0,s-max-age=0,must-revalidate")
					w.Write([]byte("ok"))
				}
				return
			}

			// Validate the access cookie.
			var secure bool
			if r.TLS != nil {
				secure = true
			}
			cookie, err := r.Cookie("access")
			if err != nil {
				w.Header().Set("Cache-Control", "no-cache,no-store,max-age=0,s-max-age=0,must-revalidate")
				http.Error(w, "gone", http.StatusGone)
				return
			}
			if !validateCookie(cookie.Value, r.UserAgent(), secret) {
				http.SetCookie(w, &http.Cookie{
					Name:     "access",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   secure,
					SameSite: http.SameSiteStrictMode,
				})
				w.Header().Set("Cache-Control", "no-cache,no-store,max-age=0,s-max-age=0,must-revalidate")
				http.Error(w, "gone", http.StatusGone)
				return
			}
			token, err := generateCookie(r.UserAgent(), ttl, secret)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     "access",
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   secure,
				SameSite: http.SameSiteStrictMode,
			})
			next.ServeHTTP(w, r)
		})
	}
}

func randomNonce(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func generateCookie(userAgent string, ttl time.Duration, secret []byte) (string, error) {
	expire := time.Now().Add(ttl).Unix()
	nonce, err := randomNonce(8)
	if err != nil {
		return "", err
	}
	payload := fmt.Sprintf(
		"%d|%s",
		expire,
		nonce,
	)
	encoded := base64.RawURLEncoding.EncodeToString(
		[]byte(payload),
	)
	toSign := encoded + "|" + userAgent
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(toSign))
	signature := hex.EncodeToString(h.Sum(nil))
	return encoded + ":" + signature, nil
}

func validateCookie(cookieValue, userAgent string, secret []byte) bool {
	parts := strings.SplitN(cookieValue, ":", 2)
	if len(parts) != 2 {
		return false
	}
	encoded := parts[0]
	signature := parts[1]
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(encoded + "|" + userAgent))
	expected := hex.EncodeToString(h.Sum(nil))
	if !hmac.Equal(
		[]byte(expected),
		[]byte(signature),
	) {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}
	payload := string(payloadBytes)
	fields := strings.SplitN(payload, "|", 2)
	if len(fields) != 2 {
		return false
	}
	expire, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() <= expire
}
