package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDirCombinesYAMLFragments(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "01-listeners.yaml"), []byte(`
listeners:
  - name: http
    type: http
    address: ":8080"
`), 0o644); err != nil {
		t.Fatalf("write listeners: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "02-routes.yaml"), []byte(`
routes:
  - name: api
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8001"
`), 0o644); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	cfg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}
	if got := len(cfg.Listeners); got != 1 {
		t.Fatalf("len(Listeners) = %d, want 1", got)
	}
	if got := cfg.Listeners[0].Name; got != "http" {
		t.Fatalf("listener name = %q, want %q", got, "http")
	}
	if got := len(cfg.Routes); got != 1 {
		t.Fatalf("len(Routes) = %d, want 1", got)
	}
	if got := cfg.Routes[0].Name; got != "api" {
		t.Fatalf("route name = %q, want %q", got, "api")
	}
}

func TestLoadDirPreservesAdminConfigAcrossFragments(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "00-admin.yaml"), []byte(`
admin:
  enabled: true
  address: ":9090"
`), 0o644); err != nil {
		t.Fatalf("write admin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "01-listeners.yaml"), []byte(`
listeners:
  - name: http
    type: http
    address: ":8080"
`), 0o644); err != nil {
		t.Fatalf("write listeners: %v", err)
	}

	cfg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}
	if !cfg.Admin.Enabled {
		t.Fatal("Admin.Enabled = false, want true")
	}
	if cfg.Admin.Address != ":9090" {
		t.Fatalf("Admin.Address = %q, want %q", cfg.Admin.Address, ":9090")
	}
}

func TestLoadDirParsesServices(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "services.yaml"), []byte(`
services:
  - name: frontend
    backends:
      - name: app1
        url: "http://127.0.0.1:8001"
      - name: app2
        url: "http://127.0.0.1:8002"
routes:
  - name: simple
    prefix: "/"
    service: frontend
`), 0o644); err != nil {
		t.Fatalf("write services: %v", err)
	}

	cfg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}
	if len(cfg.Services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(cfg.Services))
	}
	if cfg.Services[0].Name != "frontend" {
		t.Fatalf("Service name = %q, want %q", cfg.Services[0].Name, "frontend")
	}
	if len(cfg.Routes) != 1 {
		t.Fatalf("len(Routes) = %d, want 1", len(cfg.Routes))
	}
	if cfg.Routes[0].Service != "frontend" {
		t.Fatalf("Route.Service = %q, want %q", cfg.Routes[0].Service, "frontend")
	}
}

func TestWatchDirTriggersReloadOnFileChange(t *testing.T) {
	dir := t.TempDir()
	reloaded := make(chan struct{}, 1)

	if err := WatchDir(dir, func() {
		select {
		case reloaded <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("WatchDir() error = %v", err)
	}

	file := filepath.Join(dir, "watch.yaml")
	if err := os.WriteFile(file, []byte("listeners: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	select {
	case <-reloaded:
	case <-time.After(2 * time.Second):
		t.Fatal("expected reload signal after file change")
	}
}
