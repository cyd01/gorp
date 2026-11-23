package routing

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type OpenIDLoginConfig struct {
	ClientID              string
	ClientSecret          string
	RedirectURI           string
	CallbackPath          string
	AuthorizationEndpoint string
	TokenEndpoint         string
	CookieName            string
}

// OpenIDAuth validates an OpenID Connect JWT by verifying its signature and claims.
func (bm *BuiltinMiddleware) OpenIDAuth(issuer, audience, header, prefix, algorithm string, keys []string, removeHeader bool) Middleware {
	return bm.openIDAuth(issuer, audience, header, prefix, algorithm, keys, removeHeader, nil)
}

// OpenIDAuthWithLogin adds an Authorization Code + PKCE browser login flow.
func (bm *BuiltinMiddleware) OpenIDAuthWithLogin(issuer, audience, header, prefix, algorithm string, keys []string, removeHeader bool, login OpenIDLoginConfig) (Middleware, error) {
	if login.ClientID == "" || login.RedirectURI == "" ||
		login.AuthorizationEndpoint == "" || login.TokenEndpoint == "" {
		return nil, fmt.Errorf("OpenID login requires client_id, redirect_uri, authorization_endpoint, and token_endpoint")
	}
	if login.CookieName == "" {
		login.CookieName = "gorp_oidc_session"
	}
	if err := validateOpenIDLoginURLs(issuer, login); err != nil {
		return nil, err
	}
	if login.CallbackPath == "" {
		redirectURI, _ := url.Parse(login.RedirectURI)
		login.CallbackPath = redirectURI.Path
	}
	callbackPath, callbackErr := url.ParseRequestURI(login.CallbackPath)
	if callbackErr != nil || !strings.HasPrefix(login.CallbackPath, "/") ||
		strings.HasPrefix(login.CallbackPath, "//") || strings.Contains(login.CallbackPath, "\\") ||
		callbackPath.Path != login.CallbackPath || callbackPath.Host != "" || callbackPath.IsAbs() ||
		strings.ContainsAny(login.CallbackPath, "?#") {
		return nil, fmt.Errorf("OpenID callback_path must be an absolute path")
	}
	return bm.openIDAuth(issuer, audience, header, prefix, algorithm, keys, removeHeader, &login), nil
}

func (bm *BuiltinMiddleware) openIDAuth(issuer, audience, header, prefix, algorithm string, keys []string, removeHeader bool, login *OpenIDLoginConfig) Middleware {
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
			if login != nil && r.Method == http.MethodGet && r.URL.Path == login.CallbackPath {
				handleOpenIDCallback(w, r, issuer, algorithm, keys, *login)
				return
			}

			value := r.Header.Get(header)
			var token string
			sessionCookieUsed := false
			if value != "" {
				if !strings.HasPrefix(value, prefix) {
					writeOpenIDUnauthorized(w, "invalid_schema", "")
					return
				}
				token = strings.TrimSpace(strings.TrimPrefix(value, prefix))
				if err := validateOpenIDJWT(token, algorithm, keys, issuer, audience); err != nil {
					writeOpenIDUnauthorized(w, "invalid_token", err.Error())
					return
				}
			} else if login != nil {
				cookie, err := r.Cookie(login.CookieName)
				if err == nil {
					token = cookie.Value
					sessionCookieUsed = true
					if err := validateOpenIDJWT(token, algorithm, keys, issuer, login.ClientID); err != nil {
						expireOpenIDCookie(w, login.CookieName, login.RedirectURI)
						token = ""
					}
				}
			}

			if token == "" {
				if login != nil && r.Method == http.MethodGet && acceptsHTML(r) {
					if err := startOpenIDLogin(w, r, *login); err != nil {
						http.Error(w, "Unable to start OpenID login", http.StatusInternalServerError)
					}
					return
				}
				writeOpenIDUnauthorized(w, "invalid_schema", "")
				return
			}

			if sessionCookieUsed && !removeHeader {
				r.Header.Set(header, prefix+token)
			}
			if removeHeader {
				r.Header.Del(header)
			}
			if login != nil {
				removeOpenIDRequestCookies(r, *login)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validateOpenIDLoginURLs(issuer string, login OpenIDLoginConfig) error {
	issuerURL, err := url.Parse(issuer)
	if err != nil || issuerURL.Scheme == "" || issuerURL.Hostname() == "" ||
		issuerURL.User != nil || issuerURL.RawQuery != "" || issuerURL.Fragment != "" {
		return fmt.Errorf("OpenID issuer must be an absolute URL without user info, query, or fragment")
	}
	if !isSecureOrLoopbackURL(issuerURL) {
		return fmt.Errorf("OpenID issuer must use HTTPS")
	}
	redirectURI, err := url.Parse(login.RedirectURI)
	if err != nil || redirectURI.Scheme == "" || redirectURI.Hostname() == "" ||
		redirectURI.User != nil || redirectURI.Fragment != "" {
		return fmt.Errorf("OpenID redirect_uri must be an absolute URL without user info or fragment")
	}
	if !isSecureOrLoopbackURL(redirectURI) {
		return fmt.Errorf("OpenID redirect_uri must use HTTPS")
	}
	if _, err := url.ParseRequestURI(redirectURI.RequestURI()); err != nil || redirectURI.Path == "" {
		return fmt.Errorf("OpenID redirect_uri must include a callback path")
	}
	for name, raw := range map[string]string{
		"authorization_endpoint": login.AuthorizationEndpoint,
		"token_endpoint":         login.TokenEndpoint,
	} {
		endpoint, err := url.Parse(raw)
		if err != nil || endpoint.Scheme == "" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Fragment != "" {
			return fmt.Errorf("OpenID %s must be an absolute URL", name)
		}
		if !isSecureOrLoopbackURL(endpoint) {
			return fmt.Errorf("OpenID %s must use HTTPS", name)
		}
	}
	if err := (&http.Cookie{Name: login.CookieName, Value: "x"}).Valid(); err != nil {
		return fmt.Errorf("invalid OpenID session cookie name")
	}
	return nil
}

func isSecureOrLoopbackURL(parsed *url.URL) bool {
	if strings.EqualFold(parsed.Scheme, "https") {
		return true
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}

func startOpenIDLogin(w http.ResponseWriter, r *http.Request, login OpenIDLoginConfig) error {
	state, err := randomOpenIDValue(32)
	if err != nil {
		return err
	}
	nonce, err := randomOpenIDValue(32)
	if err != nil {
		return err
	}
	verifier, err := randomOpenIDValue(32)
	if err != nil {
		return err
	}
	returnTo := r.URL.RequestURI()
	if len(returnTo) > 3000 {
		return fmt.Errorf("requested URL is too long")
	}
	encodedReturn := base64.RawURLEncoding.EncodeToString([]byte(returnTo))
	cookieNames := openIDFlowCookieNames(login, state)
	setOpenIDFlowCookie(w, cookieNames.state, state, login.RedirectURI)
	setOpenIDFlowCookie(w, cookieNames.nonce, nonce, login.RedirectURI)
	setOpenIDFlowCookie(w, cookieNames.verifier, verifier, login.RedirectURI)
	setOpenIDFlowCookie(w, cookieNames.returnTo, encodedReturn, login.RedirectURI)

	challenge := sha256.Sum256([]byte(verifier))
	authorizationURL, err := url.Parse(login.AuthorizationEndpoint)
	if err != nil {
		return fmt.Errorf("parse OpenID authorization endpoint: %w", err)
	}
	query := authorizationURL.Query()
	query.Set("client_id", login.ClientID)
	query.Set("redirect_uri", login.RedirectURI)
	query.Set("response_type", "code")
	query.Set("scope", "openid profile email")
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	query.Set("code_challenge_method", "S256")
	authorizationURL.RawQuery = query.Encode()
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, authorizationURL.String(), http.StatusFound)
	return nil
}

func handleOpenIDCallback(w http.ResponseWriter, r *http.Request, issuer, algorithm string, keys []string, login OpenIDLoginConfig) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	state := query.Get("state")
	if !validOpenIDState(state) {
		http.Error(w, "Invalid OpenID login state", http.StatusBadRequest)
		return
	}
	cookieNames := openIDFlowCookieNames(login, state)
	stateCookie, stateErr := r.Cookie(cookieNames.state)
	nonceCookie, nonceErr := r.Cookie(cookieNames.nonce)
	verifierCookie, verifierErr := r.Cookie(cookieNames.verifier)
	returnCookie, returnErr := r.Cookie(cookieNames.returnTo)
	if stateErr != nil || nonceErr != nil || verifierErr != nil || returnErr != nil ||
		!hmac.Equal([]byte(stateCookie.Value), []byte(state)) {
		clearOpenIDFlowCookies(w, login.RedirectURI, cookieNames)
		http.Error(w, "Invalid OpenID login state", http.StatusBadRequest)
		return
	}
	clearOpenIDFlowCookies(w, login.RedirectURI, cookieNames)
	if query.Get("error") != "" || query.Get("code") == "" {
		http.Error(w, "OpenID login was not completed", http.StatusUnauthorized)
		return
	}

	idToken, err := exchangeOpenIDCode(r, login, query.Get("code"), verifierCookie.Value)
	if err != nil {
		http.Error(w, "OpenID token exchange failed", http.StatusBadGateway)
		return
	}
	expiry, err := validateOpenIDIDToken(idToken, algorithm, keys, issuer, login.ClientID, nonceCookie.Value)
	if err != nil {
		http.Error(w, "OpenID ID token validation failed", http.StatusUnauthorized)
		return
	}
	if len(idToken) > 3800 {
		http.Error(w, "OpenID ID token is too large for a session cookie", http.StatusBadGateway)
		return
	}

	returnToBytes, err := base64.RawURLEncoding.DecodeString(returnCookie.Value)
	if err != nil || !validOpenIDReturnTo(string(returnToBytes)) {
		http.Error(w, "Invalid OpenID return URL", http.StatusBadRequest)
		return
	}
	remaining := int(time.Until(time.Unix(expiry, 0)).Seconds())
	if remaining <= 0 {
		http.Error(w, "OpenID ID token is expired", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     login.CookieName,
		Value:    idToken,
		Path:     "/",
		Expires:  time.Unix(expiry, 0),
		MaxAge:   remaining,
		HttpOnly: true,
		Secure:   openIDRedirectIsSecure(login.RedirectURI),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, string(returnToBytes), http.StatusSeeOther)
}

func exchangeOpenIDCode(r *http.Request, login OpenIDLoginConfig, code, verifier string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {login.RedirectURI},
		"client_id":     {login.ClientID},
		"code_verifier": {verifier},
	}
	if login.ClientSecret != "" {
		form.Set("client_secret", login.ClientSecret)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, login.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create OpenID token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send OpenID token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenID token endpoint returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode OpenID token response: %w", err)
	}
	if result.IDToken == "" {
		return "", fmt.Errorf("OpenID token response has no id_token")
	}
	return result.IDToken, nil
}

func validateOpenIDIDToken(token, algorithm string, keys []string, issuer, clientID, nonce string) (int64, error) {
	if err := validateOpenIDJWT(token, algorithm, keys, issuer, clientID); err != nil {
		return 0, err
	}
	parts := strings.Split(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid ID token payload")
	}
	var claims openIDClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, fmt.Errorf("invalid ID token claims")
	}
	now := time.Now().Unix()
	if claims.Sub == "" || claims.Exp == 0 || claims.Iat == 0 {
		return 0, fmt.Errorf("ID token is missing sub, exp, or iat")
	}
	if claims.Iat > now+60 {
		return 0, fmt.Errorf("ID token issued in the future")
	}
	if claims.Nonce == "" || !hmac.Equal([]byte(claims.Nonce), []byte(nonce)) {
		return 0, fmt.Errorf("unexpected ID token nonce")
	}
	if len(claims.Aud) > 1 && claims.Azp != clientID {
		return 0, fmt.Errorf("unexpected ID token authorized party")
	}
	if claims.Azp != "" && claims.Azp != clientID {
		return 0, fmt.Errorf("unexpected ID token authorized party")
	}
	return claims.Exp, nil
}

func randomOpenIDValue(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate OpenID flow value: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func setOpenIDFlowCookie(w http.ResponseWriter, name, value, redirectURI string) {
	now := time.Now()
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  now.Add(5 * time.Minute),
		MaxAge:   300,
		HttpOnly: true,
		Secure:   openIDRedirectIsSecure(redirectURI),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOpenIDFlowCookies(w http.ResponseWriter, redirectURI string, names openIDFlowCookies) {
	for _, name := range []string{names.state, names.nonce, names.verifier, names.returnTo} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   openIDRedirectIsSecure(redirectURI),
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func expireOpenIDCookie(w http.ResponseWriter, name, redirectURI string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   openIDRedirectIsSecure(redirectURI),
		SameSite: http.SameSiteLaxMode,
	})
}

func openIDRedirectIsSecure(redirectURI string) bool {
	parsed, err := url.Parse(redirectURI)
	return err == nil && strings.EqualFold(parsed.Scheme, "https")
}

func validOpenIDReturnTo(value string) bool {
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.Contains(value, "\\") {
		return false
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && !parsed.IsAbs() && parsed.Host == "" && !strings.HasPrefix(parsed.Path, "//")
}

func acceptsHTML(r *http.Request) bool {
	for _, header := range r.Header.Values("Accept") {
		for _, accepted := range strings.Split(header, ",") {
			mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(accepted))
			if err == nil && strings.EqualFold(mediaType, "text/html") {
				quality, parseErr := strconv.ParseFloat(params["q"], 64)
				if params["q"] == "" || (parseErr == nil && quality > 0) {
					return true
				}
			}
		}
	}
	return false
}

type openIDFlowCookies struct {
	state    string
	nonce    string
	verifier string
	returnTo string
}

func openIDFlowCookieNames(login OpenIDLoginConfig, state string) openIDFlowCookies {
	prefix := login.CookieName + "_"
	return openIDFlowCookies{
		state:    prefix + "state_" + state,
		nonce:    prefix + "nonce_" + state,
		verifier: prefix + "verifier_" + state,
		returnTo: prefix + "return_" + state,
	}
}

func validOpenIDState(state string) bool {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(state)
	return err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == state
}

func removeOpenIDRequestCookies(r *http.Request, login OpenIDLoginConfig) {
	var remaining []string
	for _, cookie := range r.Cookies() {
		if cookie.Name == login.CookieName || isOpenIDFlowCookie(cookie.Name, login.CookieName) {
			continue
		}
		remaining = append(remaining, cookie.String())
	}
	if len(remaining) == 0 {
		r.Header.Del("Cookie")
	} else {
		r.Header.Set("Cookie", strings.Join(remaining, "; "))
	}
}

func isOpenIDFlowCookie(name, cookieName string) bool {
	prefix := cookieName + "_"
	return strings.HasPrefix(name, prefix+"state_") ||
		strings.HasPrefix(name, prefix+"nonce_") ||
		strings.HasPrefix(name, prefix+"verifier_") ||
		strings.HasPrefix(name, prefix+"return_")
}

func writeOpenIDUnauthorized(w http.ResponseWriter, errorCode, detail string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="OpenID", error="`+errorCode+`"`)
	if detail != "" {
		w.Header().Set("X-OpenID-Error", detail)
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
	Sub   string        `json:"sub,omitempty"`
	Iss   string        `json:"iss,omitempty"`
	Aud   claimAudience `json:"aud,omitempty"`
	Exp   int64         `json:"exp,omitempty"`
	Nbf   int64         `json:"nbf,omitempty"`
	Iat   int64         `json:"iat,omitempty"`
	Nonce string        `json:"nonce,omitempty"`
	Azp   string        `json:"azp,omitempty"`
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
