package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsUnknownConfigurationField(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(filename, []byte(`
listeners:
  - name: http
    type: http
    address: ":8080"
    typo_field: true
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(filename, true); err == nil || !strings.Contains(err.Error(), "typo_field") {
		t.Fatalf("Load() error = %v, want unknown field typo_field", err)
	}
}

func TestValidateReportsMultipleConfigurationErrors(t *testing.T) {
	cfg := &Config{
		Listeners: []Listener{{
			Name:    "https",
			Type:    "https",
			Address: ":8443",
			TLS:     TLSConfig{CertFile: "server.crt"},
			Middlewares: []MiddlewareConfig{{
				Name: "not_a_middleware",
			}},
		}},
		Routes: []Route{{
			Name:    "broken",
			Prefix:  "/",
			Service: "missing",
		}},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() succeeded for invalid configuration")
	}
	for _, issue := range []string{
		"key_file", "not_a_middleware", "unknown service",
	} {
		if !strings.Contains(err.Error(), issue) {
			t.Errorf("validation error %q does not contain %q", err, issue)
		}
	}
}

func TestValidateMockListenerScenarios(t *testing.T) {
	tests := []struct {
		name      string
		listener  Listener
		wantError string
	}{
		{
			name: "valid mock listener",
			listener: Listener{
				Name: "mock", Type: "mock", Address: ":8081",
				TLS: TLSConfig{CertFile: "server.crt", KeyFile: "server.key"},
				Middlewares: []MiddlewareConfig{{
					Name: "add_request_headers",
					Config: map[string]interface{}{
						"headers": map[string]interface{}{"X-Test": "mock"},
					},
				}},
				Mock: &MockConfig{Scenarios: map[string]MockScenario{
					"healthy": {
						Match: MockMatch{Method: "GET", Path: "/health"},
						Response: MockResponse{
							Status: 200, Headers: map[string]string{"Content-Type": "text/plain"}, Body: "ok",
						},
					},
				}},
			},
		},
		{
			name:      "mock config is required",
			listener:  Listener{Name: "mock", Type: "mock", Address: ":8081"},
			wantError: "mock is required for mock listeners",
		},
		{
			name: "invalid response status",
			listener: Listener{
				Name: "mock", Type: "mock", Address: ":8081",
				Mock: &MockConfig{Scenarios: map[string]MockScenario{
					"bad": {Response: MockResponse{Status: 700}},
				}},
			},
			wantError: "response.status must be between 200 and 599",
		},
		{
			name: "invalid default response",
			listener: Listener{
				Name: "mock", Type: "mock", Address: ":8081",
				Mock: &MockConfig{Default: &MockScenario{
					Response: MockResponse{Status: 700},
				}},
			},
			wantError: "mock.default: response.status must be between 200 and 599",
		},
		{
			name: "backends are not allowed",
			listener: Listener{
				Name: "mock", Type: "mock", Address: ":8081",
				Mock:     &MockConfig{},
				Backends: []Backend{{Name: "upstream", URL: "http://localhost:8082"}},
			},
			wantError: "mock listeners do not support backends",
		},
		{
			name: "code cannot be combined with static response",
			listener: Listener{
				Name: "mock", Type: "mock", Address: ":8081",
				Mock: &MockConfig{Scenarios: map[string]MockScenario{
					"bad": {Response: MockResponse{Code: "package dynamic", Body: "static"}},
				}},
			},
			wantError: "response.code cannot be combined",
		},
		{
			name: "path parameter names must be unique",
			listener: Listener{
				Name: "mock", Type: "mock", Address: ":8081",
				Mock: &MockConfig{Scenarios: map[string]MockScenario{
					"bad": {
						Match:    MockMatch{Path: "/users/:id/orders/:id"},
						Response: MockResponse{Body: "static"},
					},
				}},
			},
			wantError: "match.path parameter \"id\" is duplicated",
		},
		{
			name: "path parameter name is required",
			listener: Listener{
				Name: "mock", Type: "mock", Address: ":8081",
				Mock: &MockConfig{Scenarios: map[string]MockScenario{
					"bad": {
						Match:    MockMatch{Path: "/users/:"},
						Response: MockResponse{Body: "static"},
					},
				}},
			},
			wantError: "must be a non-empty identifier",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(&Config{Listeners: []Listener{test.listener}})
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestValidateMockSrvListenerRoutes(t *testing.T) {
	tests := []struct {
		name      string
		listener  Listener
		wantError string
	}{
		{
			name: "valid request match and responses",
			listener: Listener{
				Name: "mocksrv", Type: "mocksrv", Address: ":8082",
				MockSrv: &MockSrvConfig{Routes: map[string]MockSrvRoute{
					"users": {
						Match: MockMatch{
							Method:  "GET",
							Path:    "/users/*",
							Headers: map[string]string{"X-Tenant": "tenant-*"},
						},
						Scenarios: map[string]MockResponse{"found": {Status: 200, Body: "ok"}},
						Default:   &MockResponse{Status: 202, Body: "fallback"},
					},
				}},
			},
		},
		{
			name: "request match is required",
			listener: Listener{
				Name: "mocksrv", Type: "mocksrv", Address: ":8082",
				MockSrv: &MockSrvConfig{Routes: map[string]MockSrvRoute{
					"users": {Default: &MockResponse{Body: "fallback"}},
				}},
			},
			wantError: "match must define at least one of method, path, or headers",
		},
		{
			name: "invalid path wildcard",
			listener: Listener{
				Name: "mocksrv", Type: "mocksrv", Address: ":8082",
				MockSrv: &MockSrvConfig{Routes: map[string]MockSrvRoute{
					"users": {
						Match:   MockMatch{Path: "/users/["},
						Default: &MockResponse{Body: "fallback"},
					},
				}},
			},
			wantError: "match.path has an invalid wildcard pattern",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(&Config{Listeners: []Listener{test.listener}})
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestValidateRejectsInvalidSelectorAndMiddlewareTypes(t *testing.T) {
	cfg := &Config{
		Listeners: []Listener{{Name: "http", Type: "http", Address: ":8080"}},
		Routes: []Route{{
			Name:     "api",
			Prefix:   "/",
			Selector: SelectorConfig{Type: "header", Config: map[string]interface{}{"name": 42}},
			Backends: []Backend{{Name: "api", URL: "http://localhost:8081"}},
			Middlewares: []MiddlewareConfig{{
				Name:   "add_request_headers",
				Config: map[string]interface{}{"headers": "not-a-map"},
			}},
		}},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() succeeded for invalid selector and middleware values")
	}
	for _, issue := range []string{"selector.config.name", "headers must be a string map"} {
		if !strings.Contains(err.Error(), issue) {
			t.Errorf("validation error %q does not contain %q", err, issue)
		}
	}
}

func TestValidateAcceptsInteractiveOpenIDConfig(t *testing.T) {
	var problems []string
	validateMiddleware(&problems, "middleware", MiddlewareConfig{
		Name: "auth_openid",
		Config: map[string]interface{}{
			"issuer":        "https://issuer.example.com",
			"client_id":     "client",
			"client_secret": "secret",
			"redirect_uri":  "https://app.example.com/callback",
			"cookie_name":   "session",
		},
	}, false)
	if len(problems) != 0 {
		t.Fatalf("interactive OpenID config problems = %v", problems)
	}
}

func TestValidateMethodFilterMiddlewareConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		middleware MiddlewareConfig
		wantError  string
	}{
		{
			name: "allow-list requires methods",
			middleware: MiddlewareConfig{
				Name: "method_allow_list", Config: map[string]interface{}{},
			},
			wantError: "config.methods is required",
		},
		{
			name: "deny-list rejects invalid method token",
			middleware: MiddlewareConfig{
				Name:   "method_deny_list",
				Config: map[string]interface{}{"methods": []interface{}{"GET", "BAD METHOD"}},
			},
			wantError: "must be a valid HTTP method token",
		},
		{
			name: "valid method lists",
			middleware: MiddlewareConfig{
				Name:   "method_allow_list",
				Config: map[string]interface{}{"methods": []interface{}{"GET", "CUSTOM-METHOD"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				Listeners: []Listener{{Name: "http", Type: "http", Address: ":8080"}},
				Routes: []Route{{
					Name: "api", Prefix: "/api",
					Backends:    []Backend{{Name: "api", URL: "http://localhost:8081"}},
					Middlewares: []MiddlewareConfig{test.middleware},
				}},
			}
			err := Validate(cfg)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestValidateRewritePathMiddlewareConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantError string
	}{
		{
			name: "valid regex",
			config: map[string]interface{}{
				"regex": "^/old/(.*)$", "replacement": "/new/$1",
			},
		},
		{
			name: "empty replacement removes the match",
			config: map[string]interface{}{
				"regex": "^/old", "replacement": "",
			},
		},
		{
			name:      "missing regex",
			config:    map[string]interface{}{"replacement": "/new"},
			wantError: "config.regex is required",
		},
		{
			name:      "missing replacement",
			config:    map[string]interface{}{"regex": "^/old"},
			wantError: "config.replacement is required",
		},
		{
			name: "invalid regex",
			config: map[string]interface{}{
				"regex": "(", "replacement": "/new",
			},
			wantError: "config.regex is invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				Listeners: []Listener{{Name: "http", Type: "http", Address: ":8080"}},
				Routes: []Route{{
					Name: "api", Prefix: "/",
					Backends:    []Backend{{Name: "api", URL: "http://localhost:8081"}},
					Middlewares: []MiddlewareConfig{{Name: "rewrite_path", Config: test.config}},
				}},
			}
			err := Validate(cfg)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestValidateRemovePrefixMiddlewareConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantError string
	}{
		{name: "valid prefix", config: map[string]interface{}{"prefix": "/legacy"}},
		{name: "missing prefix", config: map[string]interface{}{}, wantError: "config.prefix is required"},
		{name: "invalid prefix type", config: map[string]interface{}{"prefix": 12}, wantError: "config.prefix must be a string"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				Listeners: []Listener{{Name: "http", Type: "http", Address: ":8080"}},
				Routes: []Route{{
					Name: "api", Prefix: "/",
					Backends:    []Backend{{Name: "api", URL: "http://localhost:8081"}},
					Middlewares: []MiddlewareConfig{{Name: "remove_prefix", Config: test.config}},
				}},
			}
			err := Validate(cfg)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestValidateRedirectMiddlewareConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantError string
	}{
		{
			name: "path rewrite redirect",
			config: map[string]interface{}{
				"status": 308, "regex": "^/old/(.*)$", "replacement": "/new/$1",
			},
		},
		{
			name: "host redirect",
			config: map[string]interface{}{
				"status": 301, "host": "public.example:8443",
			},
		},
		{
			name: "missing status",
			config: map[string]interface{}{
				"host": "public.example",
			},
			wantError: "config.status is required",
		},
		{
			name: "invalid status",
			config: map[string]interface{}{
				"status": 304, "host": "public.example",
			},
			wantError: "status must be one of 301",
		},
		{
			name: "missing target",
			config: map[string]interface{}{
				"status": 302,
			},
			wantError: "must define host or regex",
		},
		{
			name: "replacement without regex",
			config: map[string]interface{}{
				"status": 302, "host": "public.example", "replacement": "/new",
			},
			wantError: "replacement requires regex",
		},
		{
			name: "regex without replacement",
			config: map[string]interface{}{
				"status": 302, "regex": "^/old",
			},
			wantError: "replacement is required when regex is set",
		},
		{
			name: "invalid regex",
			config: map[string]interface{}{
				"status": 302, "regex": "(", "replacement": "/new",
			},
			wantError: "config.regex is invalid",
		},
		{
			name: "invalid host",
			config: map[string]interface{}{
				"status": 302, "host": "https://public.example/path",
			},
			wantError: "host must be a hostname",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				Listeners: []Listener{{Name: "http", Type: "http", Address: ":8080"}},
				Routes: []Route{{
					Name: "api", Prefix: "/",
					Backends:    []Backend{{Name: "api", URL: "http://localhost:8081"}},
					Middlewares: []MiddlewareConfig{{Name: "redirect", Config: test.config}},
				}},
			}
			err := Validate(cfg)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestValidateResponseThrottleMiddlewareConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantError string
	}{
		{name: "positive byte rate", config: map[string]interface{}{"bytes_per_second": 1024}},
		{name: "missing byte rate", config: map[string]interface{}{}, wantError: "config.bytes_per_second is required"},
		{name: "non-positive byte rate", config: map[string]interface{}{"bytes_per_second": 0}, wantError: "must be a positive integer"},
		{name: "non-integer byte rate", config: map[string]interface{}{"bytes_per_second": "slow"}, wantError: "must be a positive integer"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				Listeners: []Listener{{Name: "http", Type: "http", Address: ":8080"}},
				Routes: []Route{{
					Name:     "api",
					Prefix:   "/",
					Backends: []Backend{{Name: "api", URL: "http://localhost:8081"}},
					Middlewares: []MiddlewareConfig{{
						Name:   "response_throttle",
						Config: test.config,
					}},
				}},
			}
			err := Validate(cfg)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestRepositorySampleConfigPassesStrictValidation(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "tests1", "simple.yaml"), true)
	if err != nil {
		t.Fatalf("Load(sample): %v", err)
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate(sample): %v", err)
	}
}

func TestLoadMockListenerScenarios(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "mock.yaml")
	if err := os.WriteFile(filename, []byte(`
listeners:
  - name: test-api
    type: mock
    address: ":8081"
    mock:
      scenarios:
        healthy:
          match:
            method: GET
            path: /health
          response:
            status: 200
            headers:
              Content-Type: text/plain
            body: OK
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(filename, true)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got := cfg.Listeners[0].Mock.Scenarios["healthy"].Response.Body; got != "OK" {
		t.Fatalf("mock response body = %q, want OK", got)
	}
}

func TestLoadTLSKeyPassphrases(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(filename, []byte(`
admin:
  enabled: true
  address: ":9444"
  tls:
    cert_file: admin.crt
    key_file: admin.key
    key_passphrase: admin-secret
listeners:
  - name: https
    type: https
    address: ":8443"
    tls:
      cert_file: server.crt
      key_file: server.key
      key_passphrase: listener-secret
  - name: tcp
    type: tcp
    address: ":9443"
    tls:
      cert_file: tcp.crt
      key_file: tcp.key
      key_passphrase: tcp-secret
    routes:
      - name: database
        hosts: ["db.example.com"]
        tls:
          cert_file: db.crt
          key_file: db.key
          key_passphrase: route-secret
        backends:
          - name: database
            url: "tcp://localhost:5432"
routes:
  - name: api
    prefix: /
    backends:
      - name: api
        url: "https://localhost:8444"
        tls:
          cert_file: client.crt
          key_file: client.key
          key_passphrase: backend-secret
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(filename, true)
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if got := cfg.Listeners[0].TLS.KeyPassphrase; got != "listener-secret" {
		t.Errorf("listener key_passphrase = %q, want listener-secret", got)
	}
	if got := cfg.Listeners[1].TLS.KeyPassphrase; got != "tcp-secret" {
		t.Errorf("TCP listener key_passphrase = %q, want tcp-secret", got)
	}
	if got := cfg.Listeners[1].Routes[0].TLS.KeyPassphrase; got != "route-secret" {
		t.Errorf("TCP route key_passphrase = %q, want route-secret", got)
	}
	if got := cfg.Routes[0].Backends[0].TLS.KeyPassphrase; got != "backend-secret" {
		t.Errorf("backend key_passphrase = %q, want backend-secret", got)
	}
	if got := cfg.Admin.TLS.KeyPassphrase; got != "admin-secret" {
		t.Errorf("admin key_passphrase = %q, want admin-secret", got)
	}
}
