package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/cyd01/gorp/pkg/helper"

	"time"
)

type ProxyConfig struct {
	URL      string
	Username string
	Password string
}

type TransportConfig struct {
	Timeout         time.Duration
	ConnectTimeout  time.Duration
	HeadersTimeout  time.Duration
	ResponseTimeout time.Duration
	IdleTimeout     time.Duration
	Compression     bool
	Insecure        bool
	CAFile          string
	CertFile        string
	KeyFile         string
	ServerName      string
	Auth            *AuthConfig
	Proxy           *ProxyConfig
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

	responseTimeout := cfg.ResponseTimeout
	if responseTimeout <= 0 {
		responseTimeout = cfg.Timeout
	}
	if responseTimeout <= 0 {
		responseTimeout = 30 * time.Second
	}

	idleTimeout := cfg.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 90 * time.Second
	}
	tlsConfig, err := helper.BuildTLSConfigForUpstream(cfg.Insecure, cfg.ServerName, cfg.CAFile, cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("can not build TLS config")
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

	responseRoundTripper := http.RoundTripper(baseTransport)
	/*
		if responseTimeout > 0 {
			responseRoundTripper = &deadlineTransport{base: baseTransport, timeout: responseTimeout}
		}
	*/

	if cfg.Auth == nil || cfg.Auth.Type == "" {
		return responseRoundTripper, nil
	}

	return &authTransport{base: responseRoundTripper, auth: *cfg.Auth}, nil
}

type deadlineTransport struct {
	base    http.RoundTripper
	timeout time.Duration
}

func (t *deadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.timeout <= 0 {
		return t.base.RoundTrip(req)
	}
	ctx, cancel := context.WithTimeout(req.Context(), t.timeout)
	defer cancel()
	req2 := req.Clone(ctx)
	return t.base.RoundTrip(req2)
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
