package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cyd01/gorp/pkg/config"
)

func TestConnectionLimiterRejectsWhenLimitReached(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	limited := newConnectionLimiter(handler, 1, "test-listener")

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodGet, "/", nil)
	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodGet, "/", nil)

	go func() {
		limited.ServeHTTP(first, firstReq)
	}()
	<-started

	result := make(chan int, 1)
	go func() {
		limited.ServeHTTP(second, secondReq)
		result <- second.Code
	}()

	select {
	case code := <-result:
		if code != http.StatusServiceUnavailable {
			t.Fatalf("expected second request to fail with 503, got %d", code)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("second request did not return promptly")
	}

	close(release)
}

func TestRequestBodyLimiterRejectsWhenLimitReached(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})
	limited := newRequestBodyLimiter(handler, 8)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("123456789"))
	res := httptest.NewRecorder()
	limited.ServeHTTP(res, req)
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", res.Code)
	}
}

func TestResponseAffinityMiddlewareSticksToSelectedBackend(t *testing.T) {
	cfg := &config.Config{
		Routes: []config.Route{{
			Name: "app",
			Selector: config.SelectorConfig{
				Type:   "header",
				Config: map[string]interface{}{"name": "X-Backend-Affinity"},
			},
			Middlewares: []config.MiddlewareConfig{{
				Name:   "response_affinity",
				Config: map[string]interface{}{"header": "X-Backend-Affinity"},
			}},
			Backends: []config.Backend{
				{Name: "app-1", URL: "http://127.0.0.1:8001"},
				{Name: "app-2", URL: "http://127.0.0.1:8002"},
			},
		}},
	}

	routes, err := buildRoutes(cfg)
	if err != nil {
		t.Fatalf("buildRoutes() error = %v", err)
	}

	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	selected, err := routes[0].Selector.Select(firstRequest)
	if err != nil {
		t.Fatalf("initial Select() error = %v", err)
	}
	response := &http.Response{Header: make(http.Header)}
	for _, middleware := range selected.ModifyResponse {
		if err := middleware(response); err != nil {
			t.Fatalf("response middleware error = %v", err)
		}
	}

	nextRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	nextRequest.Header.Set("X-Backend-Affinity", response.Header.Get("X-Backend-Affinity"))
	nextSelected, err := routes[0].Selector.Select(nextRequest)
	if err != nil {
		t.Fatalf("subsequent Select() error = %v", err)
	}
	if nextSelected != selected {
		t.Fatalf("subsequent request selected %q, want %q", nextSelected.Name, selected.Name)
	}
}

func TestResponseAffinityMiddlewareSupportsServiceBackends(t *testing.T) {
	cfg := &config.Config{
		Services: []config.Service{{
			Name: "app-service",
			Backends: []config.Backend{
				{Name: "app-1", URL: "http://127.0.0.1:8001"},
				{Name: "app-2", URL: "http://127.0.0.1:8002"},
			},
		}},
		Routes: []config.Route{{
			Name:    "app",
			Service: "app-service",
			Selector: config.SelectorConfig{
				Type:   "header",
				Config: map[string]interface{}{"name": "X-Backend-Affinity"},
			},
			Middlewares: []config.MiddlewareConfig{{
				Name:   "response_affinity",
				Config: map[string]interface{}{"header": "X-Backend-Affinity"},
			}},
		}},
	}

	routes, err := buildRoutes(cfg)
	if err != nil {
		t.Fatalf("buildRoutes() error = %v", err)
	}

	selected, err := routes[0].Selector.Select(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("initial Select() error = %v", err)
	}
	response := &http.Response{Header: make(http.Header)}
	for _, middleware := range selected.ModifyResponse {
		if err := middleware(response); err != nil {
			t.Fatalf("response middleware error = %v", err)
		}
	}

	nextRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	nextRequest.Header.Set("X-Backend-Affinity", response.Header.Get("X-Backend-Affinity"))
	nextSelected, err := routes[0].Selector.Select(nextRequest)
	if err != nil {
		t.Fatalf("subsequent Select() error = %v", err)
	}
	if nextSelected != selected {
		t.Fatalf("subsequent request selected %q, want %q", nextSelected.Name, selected.Name)
	}
}

func TestResponseAffinityMiddlewarePreservesConfiguredOrder(t *testing.T) {
	cfg := &config.Config{
		Routes: []config.Route{{
			Name: "app",
			Middlewares: []config.MiddlewareConfig{
				{
					Name:   "response_affinity",
					Config: map[string]interface{}{"header": "X-Backend-Affinity"},
				},
				{
					Name:   "remove_response_headers",
					Config: map[string]interface{}{"headers": []interface{}{"X-Backend-Affinity"}},
				},
			},
			Backends: []config.Backend{{Name: "app-1", URL: "http://127.0.0.1:8001"}},
		}},
	}

	routes, err := buildRoutes(cfg)
	if err != nil {
		t.Fatalf("buildRoutes() error = %v", err)
	}
	selected, err := routes[0].Selector.Select(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	response := &http.Response{Header: make(http.Header)}
	for _, middleware := range selected.ModifyResponse {
		if err := middleware(response); err != nil {
			t.Fatalf("response middleware error = %v", err)
		}
	}
	if _, exists := response.Header["X-Backend-Affinity"]; exists {
		t.Fatal("remove_response_headers did not run after response_affinity")
	}
}
