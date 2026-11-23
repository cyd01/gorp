package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/helper"
	"github.com/cyd01/gorp/pkg/routing"
)

func TestServerDoesNotBecomeReadyAfterShutdown(t *testing.T) {
	srv := New(&config.Config{}, nil)
	helper.Ready.SetReady(true)

	srv.Shutdown(context.Background())
	srv.markReady()

	response := httptest.NewRecorder()
	helper.Ready.Handler(response, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("/ready status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

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

func TestBuildMiddlewareConfiguresInteractiveOpenIDLogin(t *testing.T) {
	var provider *httptest.Server
	provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"issuer":"`+provider.URL+`","authorization_endpoint":"`+provider.URL+`/authorize","token_endpoint":"`+provider.URL+`/token"}`)
	}))
	defer provider.Close()

	middlewares, err := buildMiddlewareForRoute([]config.MiddlewareConfig{{
		Name: "auth_openid",
		Config: map[string]interface{}{
			"issuer":       provider.URL,
			"client_id":    "test-client",
			"redirect_uri": "http://localhost/callback",
			"keys":         []interface{}{"public key"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	handler := middlewares[0](http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unauthenticated browser request reached the backend")
	}))
	request := httptest.NewRequest(http.MethodGet, "http://localhost/private", nil)
	request.Header.Set("Accept", "text/html")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("login response status = %d, want 302", response.Code)
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Host != strings.TrimPrefix(provider.URL, "http://") || location.Query().Get("client_id") != "test-client" {
		t.Fatalf("authorization redirect = %q", location)
	}
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

func TestMockListenerAppliesRequestMiddlewareBeforeScenarioMatching(t *testing.T) {
	listenerConfig := config.Listener{
		Name:    "mock",
		Type:    "mock",
		Address: ":8080",
		Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
			"healthy": {
				Match: config.MockMatch{
					Headers: map[string]string{"X-Mock-Injected": "yes"},
				},
				Response: config.MockResponse{Body: "ok"},
			},
		}},
		Middlewares: []config.MiddlewareConfig{{
			Name:   "add_request_headers",
			Config: map[string]interface{}{"headers": map[string]interface{}{"X-Mock-Injected": "yes"}},
		}},
	}
	srv := New(&config.Config{Listeners: []config.Listener{listenerConfig}}, nil)
	handler, err := srv.buildMockListenerHandler(listenerConfig)
	if err != nil {
		t.Fatalf("buildMockListenerHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Mock-Scenario", "healthy")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "ok" {
		t.Fatalf("mock response = %d %q, want 200 ok", res.Code, res.Body.String())
	}
}

func TestBuildRejectsInvalidMockResponseCode(t *testing.T) {
	_, err := Build(&config.Config{
		Listeners: []config.Listener{{
			Name:    "mock",
			Type:    "mock",
			Address: ":8080",
			Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
				"bad": {Response: config.MockResponse{Code: "package dynamic\nfunc Middleware("}},
			}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid mock response code") {
		t.Fatalf("Build() error = %v, want invalid mock response code", err)
	}
}

func TestBuildRejectsInvalidMockDefaultResponseCode(t *testing.T) {
	_, err := Build(&config.Config{
		Listeners: []config.Listener{{
			Name:    "mock",
			Type:    "mock",
			Address: ":8080",
			Mock: &config.MockConfig{Default: &config.MockScenario{
				Response: config.MockResponse{Code: "package dynamic\nfunc Middleware("},
			}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "default scenario") {
		t.Fatalf("Build() error = %v, want invalid default response code", err)
	}
}

func TestBuildRejectsInvalidMockSrvResponseCode(t *testing.T) {
	_, err := Build(&config.Config{
		Listeners: []config.Listener{{
			Name:    "mocksrv",
			Type:    "mocksrv",
			Address: ":8080",
			MockSrv: &config.MockSrvConfig{Routes: map[string]config.MockSrvRoute{
				"route": {
					Match: config.MockMatch{Path: "/items/*"},
					Scenarios: map[string]config.MockResponse{
						"bad": {Code: "package dynamic\nfunc Middleware("},
					},
				},
			}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid mock response code") {
		t.Fatalf("Build() error = %v, want invalid mocksrv response code", err)
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

func TestBuildRoutesSeparatesRequestAndResponseMiddlewares(t *testing.T) {
	cfg := &config.Config{
		Routes: []config.Route{{
			Name:   "api",
			Prefix: "/api",
			Backends: []config.Backend{{
				Name: "api-backend",
				URL:  "http://127.0.0.1:8001",
			}},
			Middlewares: []config.MiddlewareConfig{
				{
					Name:   "openapi",
					Config: map[string]interface{}{"file": "../../tests1/openapi.yaml"},
				},
				{
					Name:   "response_affinity",
					Config: map[string]interface{}{"header": "X-Backend-Affinity"},
				},
				{
					Name:   "response_throttle",
					Config: map[string]interface{}{"bytes_per_second": 1024},
				},
			},
		}},
	}

	routes, err := buildRoutes(cfg)
	if err != nil {
		t.Fatalf("buildRoutes() error = %v", err)
	}
	if len(routes) != 1 || len(routes[0].Middleware) != 1 {
		t.Fatalf("route middleware count = %d, want one request middleware", len(routes[0].Middleware))
	}
	selected, err := routes[0].Selector.Select(httptest.NewRequest(http.MethodGet, "/api", nil))
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if len(selected.ModifyResponse) != 2 {
		t.Fatalf("response modifier count = %d, want two response middlewares", len(selected.ModifyResponse))
	}
}

func TestBuildRoutesConfiguresPathRewriteMiddleware(t *testing.T) {
	cfg := &config.Config{
		Routes: []config.Route{{
			Name:   "api",
			Prefix: "/",
			Backends: []config.Backend{{
				Name: "api-backend",
				URL:  "http://127.0.0.1:8001",
			}},
			Middlewares: []config.MiddlewareConfig{{
				Name: "rewrite_path",
				Config: map[string]interface{}{
					"regex":       "^/legacy/(.*)$",
					"replacement": "/api/${1}",
				},
			}},
		}},
	}

	routes, err := buildRoutes(cfg)
	if err != nil {
		t.Fatalf("buildRoutes() error = %v", err)
	}
	if len(routes) != 1 || len(routes[0].Middleware) != 1 {
		t.Fatalf("route middleware count = %d, want one", len(routes[0].Middleware))
	}
	handler := routes[0].Middleware[0](http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		if got, want := request.URL.Path, "/api/items"; got != want {
			t.Errorf("URL.Path = %q, want %q", got, want)
		}
		if got, want := request.URL.RawQuery, "source=test"; got != want {
			t.Errorf("URL.RawQuery = %q, want %q", got, want)
		}
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/legacy/items?source=test", nil))
}

func TestBuildRoutesConfiguresRemovePrefixAndRedirectMiddlewares(t *testing.T) {
	cfg := &config.Config{
		Routes: []config.Route{{
			Name:   "legacy",
			Prefix: "/",
			Backends: []config.Backend{{
				Name: "legacy-backend",
				URL:  "http://127.0.0.1:8001",
			}},
			Middlewares: []config.MiddlewareConfig{
				{
					Name:   "remove_prefix",
					Config: map[string]interface{}{"prefix": "/api"},
				},
				{
					Name: "redirect",
					Config: map[string]interface{}{
						"status":      308,
						"host":        "public.example",
						"regex":       "^/legacy/(.*)$",
						"replacement": "/new/$1",
					},
				},
			},
		}},
	}

	routes, err := buildRoutes(cfg)
	if err != nil {
		t.Fatalf("buildRoutes() error = %v", err)
	}
	if len(routes) != 1 || len(routes[0].Middleware) != 2 {
		t.Fatalf("route middleware count = %d, want two", len(routes[0].Middleware))
	}
	handler := routing.ChainMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("backend handler ran despite redirect")
	}), routes[0].Middleware...)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/legacy/items?source=test", nil))
	if response.Code != http.StatusPermanentRedirect {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusPermanentRedirect)
	}
	if got, want := response.Header().Get("Location"), "http://public.example/new/items?source=test"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestHealthCheckedBackendStartsUnavailableUntilChecked(t *testing.T) {
	cfg := &config.Config{
		Routes: []config.Route{{
			Name:   "app",
			Prefix: "/",
			Backends: []config.Backend{{
				Name:        "app-1",
				URL:         "http://127.0.0.1:8001",
				HealthCheck: &config.HealthCheckConfig{},
			}},
		}},
	}

	srv, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(srv.healthCheckBackends) != 1 {
		t.Fatalf("health check backend count = %d, want 1", len(srv.healthCheckBackends))
	}
	if srv.healthCheckBackends[0].Available() {
		t.Fatal("health-checked backend should remain unavailable before its first successful check")
	}
}

func TestBuildCollectsBackendsFromRoutesAndServices(t *testing.T) {
	cfg := &config.Config{
		Services: []config.Service{{
			Name:     "app-service",
			Backends: []config.Backend{{Name: "service-backend", URL: "http://127.0.0.1:8001"}},
		}},
		Routes: []config.Route{
			{
				Name:    "service-route",
				Prefix:  "/service",
				Service: "app-service",
			},
			{
				Name:     "direct-route",
				Prefix:   "/direct",
				Backends: []config.Backend{{Name: "route-backend", URL: "http://127.0.0.1:8002"}},
			},
		},
	}

	srv, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(srv.backends) != 2 {
		t.Fatalf("backend count = %d, want 2", len(srv.backends))
	}
	if got := srv.backends[0].Name; got != "service-backend" {
		t.Errorf("first backend name = %q, want service-backend", got)
	}
	if got := srv.backends[1].Name; got != "route-backend" {
		t.Errorf("second backend name = %q, want route-backend", got)
	}
}
