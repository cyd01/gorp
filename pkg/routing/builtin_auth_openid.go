package routing

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenIDAuth validates an OpenID Connect JWT by verifying its signature and claims.
func (bm *BuiltinMiddleware) OpenIDAuth(issuer, audience, header, prefix, algorithm string, keys []string) Middleware {
	if header == "" {
		header = "Authorization"
	}
	if prefix == "" {
		prefix = "Bearer "
	}
	if algorithm == "" {
		algorithm = "RS256"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			value := r.Header.Get(header)
			if !strings.HasPrefix(value, prefix) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="OpenID", error="invalid_token"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(value, prefix))
			if err := validateOpenIDJWT(token, algorithm, keys, issuer, audience); err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer realm="OpenID", error="invalid_token"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type claimAudience []string

func (a *claimAudience) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = claimAudience{single}
		return nil
	}

	var multiple []string
	if err := json.Unmarshal(data, &multiple); err == nil {
		*a = claimAudience(multiple)
		return nil
	}

	return fmt.Errorf("invalid aud claim")
}

func (a claimAudience) Contains(target string) bool {
	for _, aud := range a {
		if aud == target {
			return true
		}
	}
	return false
}

type openIDClaims struct {
	Iss string        `json:"iss,omitempty"`
	Aud claimAudience `json:"aud,omitempty"`
	Exp int64         `json:"exp,omitempty"`
	Nbf int64         `json:"nbf,omitempty"`
	Iat int64         `json:"iat,omitempty"`
}

func validateOpenIDJWT(token, algorithm string, keys []string, issuer, audience string) error {
	if len(keys) == 0 {
		return fmt.Errorf("no signing keys configured")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid token format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("invalid token header")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("invalid token payload")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("invalid token signature")
	}

	var header struct {
		Alg string `json:"alg,omitempty"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return fmt.Errorf("invalid token header json")
	}
	if header.Alg == "" {
		header.Alg = algorithm
	}
	if !strings.EqualFold(header.Alg, algorithm) {
		return fmt.Errorf("unexpected token algorithm")
	}

	var signatureErr error
	for _, key := range keys {
		if err := verifyJWTSignature(strings.Join(parts[:2], "."), signature, header.Alg, []byte(key)); err != nil {
			signatureErr = err
			continue
		}
		return validateOpenIDClaims(payloadBytes, issuer, audience)
	}
	return signatureErr
}

func validateOpenIDClaims(payloadBytes []byte, issuer, audience string) error {
	var claims openIDClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return fmt.Errorf("invalid token claims")
	}

	now := time.Now().Unix()
	if claims.Exp != 0 && now > claims.Exp {
		return fmt.Errorf("token expired")
	}
	if claims.Nbf != 0 && now < claims.Nbf {
		return fmt.Errorf("token not valid yet")
	}
	if issuer != "" && claims.Iss != issuer {
		return fmt.Errorf("unexpected issuer")
	}
	if audience != "" && !claims.Aud.Contains(audience) {
		return fmt.Errorf("unexpected audience")
	}

	return nil
}

func verifyJWTSignature(signingInput string, signature []byte, algorithm string, key []byte) error {
	var hash crypto.Hash
	switch algorithm {
	case "RS256", "HS256":
		hash = crypto.SHA256
	case "RS384", "HS384":
		hash = crypto.SHA384
	case "RS512", "HS512":
		hash = crypto.SHA512
	default:
		return fmt.Errorf("unsupported algorithm %s", algorithm)
	}

	if !hash.Available() {
		return fmt.Errorf("hash function not available")
	}

	if strings.HasPrefix(algorithm, "RS") {
		pub, err := parseRSAPublicKey(key)
		if err != nil {
			return err
		}
		h := hash.New()
		h.Write([]byte(signingInput))
		digest := h.Sum(nil)
		return rsa.VerifyPKCS1v15(pub, hash, digest, signature)
	}

	mac := hmac.New(hash.New, key)
	mac.Write([]byte(signingInput))
	if !hmac.Equal(mac.Sum(nil), signature) {
		return fmt.Errorf("invalid token signature")
	}

	return nil
}

func parseRSAPublicKey(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM data")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		if rsaPub, ok := pub.(*rsa.PublicKey); ok {
			return rsaPub, nil
		}
	}

	rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err == nil {
		return rsaPub, nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err == nil {
		if rsaPub, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return rsaPub, nil
		}
	}

	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("unsupported public key type")
}
