package backend

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cyd01/gorp/pkg/helper"
)

type ProxyConfig struct {
	URL      string
	Username string
	Password string
}

type TransportConfig struct {
	Timeout        time.Duration
	ConnectTimeout time.Duration
	HeadersTimeout time.Duration
	IdleTimeout    time.Duration
	Retries        int
	RetryBackoff   time.Duration
	Compression    bool
	Insecure       bool
	CAFile         string
	CertFile       string
	KeyFile        string
	KeyPassphrase  string
	ServerName     string
	Auth           *AuthConfig
	Proxy          *ProxyConfig
}

type AuthConfig struct {
	Type            string
	Username        string
	Password        string
	Token           string
	Header          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Service         string
}

func NewTransport(cfg TransportConfig) (http.RoundTripper, error) {
	if cfg.Retries < 0 || cfg.Retries > 5 {
		return nil, fmt.Errorf("retries must be between 0 and 5")
	}
	if cfg.RetryBackoff < 0 {
		return nil, fmt.Errorf("retry backoff must not be negative")
	}
	if cfg.Retries > 0 && cfg.RetryBackoff == 0 {
		cfg.RetryBackoff = 100 * time.Millisecond
	}

	connectTimeout := cfg.ConnectTimeout
	if connectTimeout <= 0 {
		connectTimeout = cfg.Timeout
	}
	if connectTimeout <= 0 {
		connectTimeout = 30 * time.Second
	}

	headersTimeout := cfg.HeadersTimeout
	if headersTimeout <= 0 {
		headersTimeout = cfg.Timeout
	}
	if headersTimeout <= 0 {
		headersTimeout = 30 * time.Second
	}

	idleTimeout := cfg.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 90 * time.Second
	}
	tlsConfig, err := helper.BuildTLSConfigForUpstream(cfg.Insecure, cfg.ServerName, cfg.CAFile, cfg.CertFile, cfg.KeyFile, cfg.KeyPassphrase)
	if err != nil {
		return nil, fmt.Errorf("cannot build TLS config: %w", err)
	}
	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: connectTimeout,
		}).DialContext,
		TLSClientConfig:       tlsConfig,
		ResponseHeaderTimeout: headersTimeout,
		IdleConnTimeout:       idleTimeout,
		DisableCompression:    !cfg.Compression,
		ForceAttemptHTTP2:     true,
	}

	if cfg.Proxy != nil && cfg.Proxy.URL != "" {
		proxyURL, err := url.Parse(cfg.Proxy.URL)
		if err != nil {
			return nil, err
		}
		if proxyURL.User == nil && cfg.Proxy.Username != "" {
			proxyURL.User = url.UserPassword(cfg.Proxy.Username, cfg.Proxy.Password)
		}
		baseTransport.Proxy = http.ProxyURL(proxyURL)
	}

	var responseRoundTripper http.RoundTripper = baseTransport
	if cfg.Auth != nil && cfg.Auth.Type != "" {
		responseRoundTripper = &authTransport{base: responseRoundTripper, auth: *cfg.Auth}
	}
	if cfg.Retries > 0 {
		responseRoundTripper = &retryTransport{
			base:    responseRoundTripper,
			retries: cfg.Retries,
			backoff: cfg.RetryBackoff,
		}
	}
	return responseRoundTripper, nil
}

type retryTransport struct {
	base    http.RoundTripper
	retries int
	backoff time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return t.base.RoundTrip(req)
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}

	replayableBody := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
	attempts := t.retries + 1
	if !replayableBody {
		attempts = 1
	}

	for attempt := 0; attempt < attempts; attempt++ {
		attemptRequest := req.Clone(req.Context())
		if attempt > 0 && req.Body != nil && req.Body != http.NoBody {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("recreate request body for retry: %w", err)
			}
			attemptRequest.Body = body
		}

		response, err := t.base.RoundTrip(attemptRequest)
		if attempt == attempts-1 || !retryableResponse(response, err) {
			return response, err
		}
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		if contextErr := req.Context().Err(); contextErr != nil {
			return nil, contextErr
		}

		delay := t.backoff
		if delay > time.Second {
			delay = time.Second
		}
		for backoff := 0; backoff < attempt && delay < time.Second; backoff++ {
			if delay > time.Second/2 {
				delay = time.Second
			} else {
				delay *= 2
			}
		}
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}

	return nil, errors.New("HTTP retry attempts exhausted")
}

func retryableResponse(response *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if response == nil {
		return false
	}
	switch response.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

type authTransport struct {
	base http.RoundTripper
	auth AuthConfig
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header = req.Header.Clone()

	switch strings.ToLower(t.auth.Type) {
	case "basic":
		encoded := base64.StdEncoding.EncodeToString([]byte(t.auth.Username + ":" + t.auth.Password))
		req2.Header.Set("Authorization", "Basic "+encoded)
	case "token":
		header := t.auth.Header
		if header == "" {
			header = "Authorization"
		}
		prefix := t.auth.Prefix
		if prefix == "" {
			prefix = "Bearer "
		}
		req2.Header.Set(header, prefix+t.auth.Token)
	case "aws":
		signer := &AWSSignerV4{
			AccessKeyID:     t.auth.AccessKeyID,
			SecretAccessKey: t.auth.SecretAccessKey,
			Region:          t.auth.Region,
			Service:         t.auth.Service,
		}
		if err := signer.SignRequest(req2); err != nil {
			return nil, err
		}
	}
	return t.base.RoundTrip(req2)
}
