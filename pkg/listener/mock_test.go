package listener

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cyd01/gorp/pkg/config"
)

func TestMockHandlerServesScenarioAndRequestOverrides(t *testing.T) {
	registry := NewMockRegistry([]config.Listener{{
		Name: "test-api",
		Type: "mock",
		Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
			"created": {
				Match: config.MockMatch{Method: http.MethodPost, Path: "/orders"},
				Response: config.MockResponse{
					Status:  http.StatusCreated,
					Headers: map[string]string{"X-Source": "scenario"},
					Body:    `{"id":1}`,
				},
			},
		}},
	}})
	handler := registry.Handler("test-api")

	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader("request"))
	req.Header.Set(mockScenarioHeader, "created")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || res.Body.String() != `{"id":1}` {
		t.Fatalf("scenario response = %d %q, want 201 with configured body", res.Code, res.Body.String())
	}
	if got := res.Header().Get("X-Source"); got != "scenario" {
		t.Fatalf("X-Source = %q, want scenario", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader("request"))
	req.Header.Set(mockScenarioHeader, "created")
	req.Header.Set(mockStatusHeader, "502")
	req.Header.Set(mockBodyHeader, "temporary failure")
	req.Header.Set(mockResponseHeaderPrefix+"X-Source", "request")
	req.Header.Set(mockResponseHeaderPrefix+"X-Extra", "value")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadGateway || res.Body.String() != "temporary failure" {
		t.Fatalf("override response = %d %q, want 502 with override body", res.Code, res.Body.String())
	}
	if got := res.Header().Get("X-Source"); got != "request" {
		t.Fatalf("overridden X-Source = %q, want request", got)
	}
	if got := res.Header().Get("X-Extra"); got != "value" {
		t.Fatalf("X-Extra = %q, want value", got)
	}
}

func TestMockHandlerExecutesDynamicResponseCode(t *testing.T) {
	registry := NewMockRegistry([]config.Listener{{
		Name: "test-api",
		Type: "mock",
		Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
			"dynamic": {
				Response: config.MockResponse{Code: `package dynamic
import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func Middleware(response *http.Response) error {
	response.StatusCode = http.StatusCreated
	response.Header.Set("Content-Type", "application/json")
	response.Header.Set("X-Path", response.Request.URL.Path)
	response.Body = io.NopCloser(strings.NewReader(fmt.Sprintf("{\"name\":%q}", response.Request.URL.Query().Get("name"))))
	return nil
}`,
				},
			},
		}},
	}})
	if err := registry.Err(); err != nil {
		t.Fatalf("compile mock response code: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/users?name=Ada", nil)
	req.Header.Set(mockScenarioHeader, "dynamic")
	res := httptest.NewRecorder()
	registry.Handler("test-api").ServeHTTP(res, req)
	if res.Code != http.StatusCreated || res.Body.String() != `{"name":"Ada"}` {
		t.Fatalf("generated response = %d %q, want 201 with generated body", res.Code, res.Body.String())
	}
	if got := res.Header().Get("X-Path"); got != "/users" {
		t.Fatalf("generated X-Path = %q, want /users", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/users?name=Ada", nil)
	req.Header.Set(mockScenarioHeader, "dynamic")
	req.Header.Set(mockStatusHeader, "202")
	req.Header.Set(mockBodyHeader, "overridden")
	req.Header.Set(mockResponseHeaderPrefix+"X-Path", "/override")
	res = httptest.NewRecorder()
	registry.Handler("test-api").ServeHTTP(res, req)
	if res.Code != http.StatusAccepted || res.Body.String() != "overridden" {
		t.Fatalf("dynamic response = %d %q, want 202 overridden", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := res.Header().Get("X-Path"); got != "/override" {
		t.Fatalf("X-Path = %q, want override", got)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/users?name=user-%d", index), nil)
			req.Header.Set(mockScenarioHeader, "dynamic")
			res := httptest.NewRecorder()
			registry.Handler("test-api").ServeHTTP(res, req)
			if res.Code != http.StatusCreated || res.Body.String() != fmt.Sprintf(`{"name":"user-%d"}`, index) {
				errs <- fmt.Errorf("concurrent response = %d %q for request %d", res.Code, res.Body.String(), index)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestMockHandlerCapturesPathParametersForDynamicResponse(t *testing.T) {
	registry := NewMockRegistry([]config.Listener{{
		Name: "test-api",
		Type: "mock",
		Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
			"user": {
				Match: config.MockMatch{Path: "/teams/:team/users/:id"},
				Response: config.MockResponse{Code: `package dynamic
import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

func Middleware(response *http.Response) error {
	body, err := json.Marshal(map[string]string{
		"team": response.Request.PathValue("team"),
		"id": response.Request.PathValue("id"),
	})
	if err != nil {
		return err
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}`,
				},
			},
		}},
	}})
	if err := registry.Err(); err != nil {
		t.Fatalf("compile mock response code: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/teams/platform/users/42", nil)
	req.Header.Set(mockScenarioHeader, "user")
	res := httptest.NewRecorder()
	registry.Handler("test-api").ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != `{"id":"42","team":"platform"}` {
		t.Fatalf("mock response = %d %q, want path parameters in generated body", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/teams/platform/users/", nil)
	req.Header.Set(mockScenarioHeader, "user")
	res = httptest.NewRecorder()
	registry.Handler("test-api").ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("empty path parameter status = %d, want 404", res.Code)
	}
}

func TestMockHandlerRejectsDynamicResponseErrors(t *testing.T) {
	registry := NewMockRegistry([]config.Listener{{
		Name: "test-api",
		Type: "mock",
		Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
			"invalid-status": {
				Response: config.MockResponse{Code: `package dynamic
import "net/http"
func Middleware(response *http.Response) error {
	response.StatusCode = 700
	return nil
}`,
				},
			},
		}},
	}})
	if err := registry.Err(); err != nil {
		t.Fatalf("compile mock response code: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(mockScenarioHeader, "invalid-status")
	res := httptest.NewRecorder()
	registry.Handler("test-api").ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("dynamic response status = %d, want 500", res.Code)
	}
}

func TestMockHandlerTurnsDynamicResponsePanicIntoInternalServerError(t *testing.T) {
	registry := NewMockRegistry([]config.Listener{{
		Name: "test-api",
		Type: "mock",
		Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
			"invalid-body": {
				Response: config.MockResponse{Code: `package dynamic
import (
	"encoding/json"
	"net/http"
)

func Middleware(response *http.Response) error {
	var err error
	response.Body, err = json.Marshal(map[string]string{"id": "42"})
	return err
}`,
				},
			},
		}},
	}})
	if err := registry.Err(); err != nil {
		t.Fatalf("compile mock response code: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(mockScenarioHeader, "invalid-body")
	res := httptest.NewRecorder()
	registry.Handler("test-api").ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("dynamic response status = %d, want 500", res.Code)
	}
	if !strings.Contains(res.Body.String(), "panic while executing response code") {
		t.Fatalf("dynamic response error = %q, want panic details", res.Body.String())
	}
}

func TestMockHandlerRejectsMissingUnknownAndMismatchedScenarios(t *testing.T) {
	registry := NewMockRegistry([]config.Listener{{
		Name: "test-api",
		Type: "mock",
		Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
			"get-user": {
				Match:    config.MockMatch{Method: http.MethodGet, Path: "/users/1"},
				Response: config.MockResponse{Body: "user"},
			},
		}},
	}})
	handler := registry.Handler("test-api")

	tests := []struct {
		name       string
		method     string
		path       string
		scenario   string
		wantStatus int
	}{
		{name: "missing selector", method: http.MethodGet, path: "/users/1", wantStatus: http.StatusBadRequest},
		{name: "unknown scenario", method: http.MethodGet, path: "/users/1", scenario: "missing", wantStatus: http.StatusNotFound},
		{name: "mismatched path", method: http.MethodGet, path: "/users/2", scenario: "get-user", wantStatus: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			if test.scenario != "" {
				req.Header.Set(mockScenarioHeader, test.scenario)
			}
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", res.Code, test.wantStatus)
			}
		})
	}
}

func TestMockHandlerSelectsScenarioWithWildcards(t *testing.T) {
	handler := NewMockRegistry([]config.Listener{{
		Name: "test-api",
		Type: "mock",
		Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
			"user-dynamic": {Response: config.MockResponse{Body: "dynamic"}},
			"user-static":  {Response: config.MockResponse{Body: "static"}},
			"literal-*":    {Response: config.MockResponse{Body: "literal"}},
		}},
	}}).Handler("test-api")

	tests := []struct {
		name       string
		scenario   string
		wantStatus int
		wantBody   string
	}{
		{name: "star matches one scenario", scenario: "user-dyn*", wantStatus: http.StatusOK, wantBody: "dynamic"},
		{name: "question mark matches one character", scenario: "user-d?namic", wantStatus: http.StatusOK, wantBody: "dynamic"},
		{name: "exact name wins over wildcard matches", scenario: "literal-*", wantStatus: http.StatusOK, wantBody: "literal"},
		{name: "ambiguous pattern is rejected", scenario: "user-*", wantStatus: http.StatusBadRequest, wantBody: "X-Mock-Scenario matches multiple scenarios\n"},
		{name: "unmatched pattern is not found", scenario: "order-*", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(mockScenarioHeader, test.scenario)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != test.wantStatus || res.Body.String() != test.wantBody {
				t.Fatalf("response = %d %q, want %d %q", res.Code, res.Body.String(), test.wantStatus, test.wantBody)
			}
		})
	}
}

func TestMockHandlerRejectsInvalidRequestOverrides(t *testing.T) {
	handler := NewMockRegistry([]config.Listener{{
		Name: "test-api",
		Type: "mock",
		Mock: &config.MockConfig{Scenarios: map[string]config.MockScenario{
			"anything": {Response: config.MockResponse{Body: "configured"}},
		}},
	}}).Handler("test-api")

	for _, header := range []struct {
		name  string
		value string
	}{
		{name: mockStatusHeader, value: "700"},
		{name: mockResponseHeaderPrefix + "Content-Length", value: "100"},
		{name: mockResponseHeaderPrefix + "Bad Header", value: "value"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(mockScenarioHeader, "anything")
		req.Header.Set(header.name, header.value)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", header.name, res.Code)
		}
	}
}

func TestMockAdminScenarioLifecycle(t *testing.T) {
	registry := NewMockRegistry([]config.Listener{{Name: "test-api", Type: "mock"}})
	admin, err := NewAdminServerWithBackendsAndMocksAndKeyPassphrase(
		":0", "", "", "", "", "", "", &config.Config{}, nil, nil, registry,
	)
	if err != nil {
		t.Fatalf("create admin server: %v", err)
	}

	body := `{"match":{"method":"GET","path":"/health"},"response":{"status":200,"headers":{"Content-Type":"text/plain"},"body":"ok"}}`
	req := httptest.NewRequest(http.MethodPut, "/mocks/test-api/scenarios/healthy", strings.NewReader(body))
	res := httptest.NewRecorder()
	admin.Handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("PUT scenario status = %d, want 204: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/mocks/test-api/scenarios", nil)
	res = httptest.NewRecorder()
	admin.Handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("GET scenarios status = %d, want 200", res.Code)
	}
	var scenarios map[string]config.MockScenario
	if err := json.Unmarshal(res.Body.Bytes(), &scenarios); err != nil {
		t.Fatalf("decode scenarios: %v", err)
	}
	if got := scenarios["healthy"].Response.Body; got != "ok" {
		t.Fatalf("scenario body = %q, want ok", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(mockScenarioHeader, "healthy")
	res = httptest.NewRecorder()
	registry.Handler("test-api").ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "ok" {
		t.Fatalf("mock response = %d %q, want 200 ok", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/mocks/test-api/scenarios/healthy", nil)
	res = httptest.NewRecorder()
	admin.Handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("DELETE scenario status = %d, want 204", res.Code)
	}
}

func TestMockAdminRejectsInvalidScenario(t *testing.T) {
	registry := NewMockRegistry([]config.Listener{{Name: "test-api", Type: "mock"}})
	admin, err := NewAdminServerWithBackendsAndMocksAndKeyPassphrase(
		":0", "", "", "", "", "", "", &config.Config{}, nil, nil, registry,
	)
	if err != nil {
		t.Fatalf("create admin server: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/mocks/test-api/scenarios/bad", strings.NewReader(
		`{"response":{"status":700,"body":"invalid"}}`,
	))
	res := httptest.NewRecorder()
	admin.Handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("PUT invalid scenario status = %d, want 400", res.Code)
	}
}

func TestMockAdminRejectsInvalidDynamicResponseCode(t *testing.T) {
	registry := NewMockRegistry([]config.Listener{{Name: "test-api", Type: "mock"}})
	admin, err := NewAdminServerWithBackendsAndMocksAndKeyPassphrase(
		":0", "", "", "", "", "", "", &config.Config{}, nil, nil, registry,
	)
	if err != nil {
		t.Fatalf("create admin server: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/mocks/test-api/scenarios/bad", strings.NewReader(
		`{"response":{"code":"package dynamic\\nfunc Middleware( {"}}`,
	))
	res := httptest.NewRecorder()
	admin.Handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("PUT invalid response code status = %d, want 400: %s", res.Code, res.Body.String())
	}
}
