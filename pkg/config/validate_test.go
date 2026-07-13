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

	if _, err := Load(filename); err == nil || !strings.Contains(err.Error(), "typo_field") {
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

func TestRepositorySampleConfigPassesStrictValidation(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "tests1", "simple.yaml"))
	if err != nil {
		t.Fatalf("Load(sample): %v", err)
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate(sample): %v", err)
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

	cfg, err := Load(filename)
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
