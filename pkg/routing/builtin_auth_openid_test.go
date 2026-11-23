package routing

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOpenIDLoginFlowCreatesSession(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER}))

	var expectedChallenge string
	var expectedNonce string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse token request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if got := r.Form.Get("redirect_uri"); got != "https://app.example/callback" {
			t.Errorf("redirect_uri = %q", got)
		}
		if got := r.Form.Get("client_id"); got != "test-client" {
			t.Errorf("client_id = %q", got)
		}
		challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if got := base64.RawURLEncoding.EncodeToString(challenge[:]); got != expectedChallenge {
			t.Errorf("PKCE challenge = %q, want %q", got, expectedChallenge)
		}
		idToken := signTestIDToken(t, privateKey, "https://issuer.example", "test-client", expectedNonce)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id_token":%q}`, idToken)
	}))
	defer provider.Close()

	login := OpenIDLoginConfig{
		ClientID:              "test-client",
		RedirectURI:           "https://app.example/callback",
		AuthorizationEndpoint: provider.URL + "/authorize",
		TokenEndpoint:         provider.URL + "/token",
		CookieName:            "gorp_oidc_session",
	}
	middleware, err := NewBuiltinMiddleware().OpenIDAuthWithLogin(
		"https://issuer.example", "", "Authorization", "Bearer ", "RS256",
		[]string{publicKey}, false, login,
	)
	if err != nil {
		t.Fatal(err)
	}

	var receivedAuthorization string
	var receivedCookies string
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthorization = r.Header.Get("Authorization")
		receivedCookies = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusNoContent)
	}))

	apiRequest := httptest.NewRequest(http.MethodGet, "https://app.example/private", nil)
	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusUnauthorized {
		t.Fatalf("API request status = %d, want 401", apiResponse.Code)
	}

	loginRequest := httptest.NewRequest(http.MethodGet, "https://app.example/private?tab=one&state=app-state", nil)
	loginRequest.Header.Set("Accept", "text/html")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusFound {
		t.Fatalf("login status = %d, want 302", loginResponse.Code)
	}
	location, err := url.Parse(loginResponse.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	expectedChallenge = location.Query().Get("code_challenge")
	state := location.Query().Get("state")
	expectedNonce = location.Query().Get("nonce")
	if state == "" || expectedNonce == "" || expectedChallenge == "" {
		t.Fatal("authorization redirect is missing state, nonce, or PKCE challenge")
	}

	callbackRequest := httptest.NewRequest(http.MethodGet, "https://app.example/callback?code=auth-code&state="+url.QueryEscape(state), nil)
	for _, cookie := range loginResponse.Result().Cookies() {
		callbackRequest.AddCookie(cookie)
	}
	mismatchedStateRequest := callbackRequest.Clone(callbackRequest.Context())
	mismatchedStateRequest.URL = mustParseTestURL(t, "https://app.example/callback?code=auth-code&state="+strings.Repeat("A", len(state)))
	mismatchedStateResponse := httptest.NewRecorder()
	handler.ServeHTTP(mismatchedStateResponse, mismatchedStateRequest)
	if mismatchedStateResponse.Code != http.StatusBadRequest {
		t.Fatalf("mismatched state status = %d, want 400", mismatchedStateResponse.Code)
	}

	callbackResponse := httptest.NewRecorder()
	handler.ServeHTTP(callbackResponse, callbackRequest)
	if callbackResponse.Code != http.StatusSeeOther {
		t.Fatalf("callback status = %d, want 303; body: %s", callbackResponse.Code, callbackResponse.Body.String())
	}
	if got := callbackResponse.Header().Get("Location"); got != "/private?tab=one&state=app-state" {
		t.Fatalf("callback redirect = %q, want original URL", got)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range callbackResponse.Result().Cookies() {
		if cookie.Name == login.CookieName {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie has unexpected attributes: %#v", sessionCookie)
	}

	sessionRequest := httptest.NewRequest(http.MethodGet, "https://app.example/private?state=app-state", nil)
	sessionRequest.AddCookie(sessionCookie)
	sessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusNoContent {
		t.Fatalf("session request status = %d, want 204", sessionResponse.Code)
	}
	if !strings.HasPrefix(receivedAuthorization, "Bearer ") {
		t.Fatalf("Authorization header was not forwarded from session: %q", receivedAuthorization)
	}
	if strings.Contains(receivedCookies, login.CookieName) {
		t.Fatalf("session cookie was forwarded to backend: %q", receivedCookies)
	}
}

func TestOpenIDLoginRequiresBrowserAcceptHeader(t *testing.T) {
	middleware, err := NewBuiltinMiddleware().OpenIDAuthWithLogin(
		"https://issuer.example", "", "Authorization", "Bearer ", "RS256", []string{"key"}, false,
		OpenIDLoginConfig{
			ClientID:              "test-client",
			RedirectURI:           "https://app.example/callback",
			AuthorizationEndpoint: "https://issuer.example/authorize",
			TokenEndpoint:         "https://issuer.example/token",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	request := httptest.NewRequest(http.MethodGet, "https://app.example/private", nil)
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("API-like request status = %d, want 401", response.Code)
	}
}

func signTestIDToken(t *testing.T, key *rsa.PrivateKey, issuer, audience, nonce string) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]interface{}{
		"sub":   "user-123",
		"iss":   issuer,
		"aud":   audience,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"nonce": nonce,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func mustParseTestURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
