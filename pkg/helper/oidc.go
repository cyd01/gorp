package helper

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type discoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type OpenIDProviderMetadata struct {
	Issuer                string
	AuthorizationEndpoint string
	TokenEndpoint         string
	JWKSURI               string
}

type jwksDocument struct {
	Keys []json.RawMessage `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
	Kid string `json:"kid,omitempty"`

	// RSA
	N string `json:"n,omitempty"`
	E string `json:"e,omitempty"`

	// EC
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`

	// OKP / Ed25519
	// For Ed25519, x contains the public key.
}

func GetJWKSPEM(providerURL string) ([]string, error) {
	discovery, err := getOpenIDDiscovery(providerURL)
	if err != nil {
		return nil, err
	}

	if discovery.JWKSURI == "" {
		return nil, errors.New("discovery document does not contain jwks_uri")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, discovery.JWKSURI, nil)
	if err != nil {
		return nil, fmt.Errorf("create JWKS request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query JWKS endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"JWKS endpoint returned HTTP %d",
			resp.StatusCode,
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read JWKS response: %w", err)
	}

	var jwks jwksDocument

	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("decode JWKS document: %w", err)
	}

	keys := make([]string, 0, len(jwks.Keys))

	for i, rawKey := range jwks.Keys {
		var key jwk

		if err := json.Unmarshal(rawKey, &key); err != nil {
			return nil, fmt.Errorf(
				"decode JWK %d: %w",
				i,
				err,
			)
		}

		pemKey, err := jwkToPEM(&key)
		if err != nil {
			return nil, fmt.Errorf(
				"convert JWK %d (kid=%q) to PEM: %w",
				i,
				key.Kid,
				err,
			)
		}

		keys = append(keys, pemKey)
	}

	return keys, nil
}

func GetOpenIDProviderMetadata(providerURL string) (OpenIDProviderMetadata, error) {
	parsed, err := url.Parse(providerURL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return OpenIDProviderMetadata{}, fmt.Errorf("OpenID issuer must be an absolute URL without user info, query, or fragment")
	}
	host := strings.ToLower(parsed.Hostname())
	ip := net.ParseIP(host)
	loopback := host == "localhost" || ip != nil && ip.IsLoopback()
	if !strings.EqualFold(parsed.Scheme, "https") && !(strings.EqualFold(parsed.Scheme, "http") && loopback) {
		return OpenIDProviderMetadata{}, fmt.Errorf("OpenID issuer must use HTTPS")
	}
	discovery, err := getOpenIDDiscovery(providerURL)
	if err != nil {
		return OpenIDProviderMetadata{}, err
	}
	return OpenIDProviderMetadata{
		Issuer:                discovery.Issuer,
		AuthorizationEndpoint: discovery.AuthorizationEndpoint,
		TokenEndpoint:         discovery.TokenEndpoint,
		JWKSURI:               discovery.JWKSURI,
	}, nil
}

func getOpenIDDiscovery(providerURL string) (discoveryDocument, error) {
	discoveryURL := strings.TrimRight(providerURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequest(http.MethodGet, discoveryURL, nil)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("create discovery request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("query discovery endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return discoveryDocument{}, fmt.Errorf("discovery endpoint returned HTTP %d", resp.StatusCode)
	}

	var discovery discoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return discoveryDocument{}, fmt.Errorf("decode discovery document: %w", err)
	}
	return discovery, nil
}

func jwkToPEM(key *jwk) (string, error) {
	switch key.Kty {
	case "RSA":
		return rsaJWKToPEM(key)

	case "EC":
		return ecJWKToPEM(key)

	case "OKP":
		return okpJWKToPEM(key)

	default:
		return "", fmt.Errorf(
			"unsupported JWK key type %q",
			key.Kty,
		)
	}
}

// ------------------------------------------------------------
// RSA
// ------------------------------------------------------------

func rsaJWKToPEM(key *jwk) (string, error) {
	if key.N == "" {
		return "", errors.New("RSA key has no modulus")
	}

	if key.E == "" {
		return "", errors.New("RSA key has no exponent")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return "", fmt.Errorf("decode RSA modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return "", fmt.Errorf("decode RSA exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)

	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}

	if e == 0 {
		return "", errors.New("invalid RSA exponent")
	}

	publicKey := &rsa.PublicKey{
		N: n,
		E: e,
	}

	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("marshal RSA public key: %w", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})), nil
}

// ------------------------------------------------------------
// EC
// ------------------------------------------------------------

func ecJWKToPEM(key *jwk) (string, error) {
	if key.Crv == "" {
		return "", errors.New("EC key has no curve")
	}

	var curve elliptic.Curve

	switch key.Crv {
	case "P-256":
		curve = elliptic.P256()

	case "P-384":
		curve = elliptic.P384()

	case "P-521":
		curve = elliptic.P521()

	default:
		return "", fmt.Errorf(
			"unsupported EC curve %q",
			key.Crv,
		)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil {
		return "", fmt.Errorf("decode EC X coordinate: %w", err)
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(key.Y)
	if err != nil {
		return "", fmt.Errorf("decode EC Y coordinate: %w", err)
	}

	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)

	if !curve.IsOnCurve(x, y) {
		return "", errors.New("EC public key is not on the specified curve")
	}

	publicKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}

	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("marshal EC public key: %w", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})), nil
}

// ------------------------------------------------------------
// Ed25519
// ------------------------------------------------------------

func okpJWKToPEM(key *jwk) (string, error) {
	if key.Crv != "Ed25519" {
		return "", fmt.Errorf(
			"unsupported OKP curve %q",
			key.Crv,
		)
	}

	if key.X == "" {
		return "", errors.New("Ed25519 key has no public key value")
	}

	x, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil {
		return "", fmt.Errorf(
			"decode Ed25519 public key: %w",
			err,
		)
	}

	if len(x) != ed25519.PublicKeySize {
		return "", fmt.Errorf(
			"invalid Ed25519 public key size: %d",
			len(x),
		)
	}

	publicKey := ed25519.PublicKey(x)

	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf(
			"marshal Ed25519 public key: %w",
			err,
		)
	}

	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})), nil
}
