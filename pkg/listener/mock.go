package listener

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/dynamic"
	"github.com/cyd01/gorp/pkg/helper"
	"github.com/cyd01/gorp/pkg/routing"
)

const (
	mockScenarioHeader       = "X-Mock-Scenario"
	mockStatusHeader         = "X-Mock-Status"
	mockBodyHeader           = "X-Mock-Body"
	mockResponseHeaderPrefix = "X-Mock-Response-Header-"
	maxMockScenarioBody      = 1 << 20
)

// MockRegistry stores the scenarios for all configured mock listeners.
type MockRegistry struct {
	mu        sync.RWMutex
	listeners map[string]*MockHandler
}

// MockHandler serves responses from named scenarios without contacting an upstream.
type MockHandler struct {
	mu        sync.RWMutex
	scenarios map[string]compiledMockScenario
	errors    []error
}

type compiledMockScenario struct {
	scenario config.MockScenario
	modifier func(*http.Response) error
	err      error
}

// NewMockRegistry creates a registry initialized with the configured scenarios.
func NewMockRegistry(listeners []config.Listener) *MockRegistry {
	registry := &MockRegistry{listeners: make(map[string]*MockHandler)}
	for _, l := range listeners {
		if l.Type != "mock" {
			continue
		}
		handler := &MockHandler{scenarios: make(map[string]compiledMockScenario)}
		if l.Mock != nil {
			for name, scenario := range l.Mock.Scenarios {
				compiled, err := compileMockScenario(scenario)
				if err != nil {
					handler.errors = append(handler.errors, fmt.Errorf("listener %q scenario %q: %w", l.Name, name, err))
				}
				handler.scenarios[name] = compiled
			}
		}
		registry.listeners[l.Name] = handler
	}
	return registry
}

// Err returns any errors encountered while compiling configured response code.
func (r *MockRegistry) Err() error {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var errs []error
	for _, handler := range r.listeners {
		handler.mu.RLock()
		for _, err := range handler.errors {
			errs = append(errs, err)
		}
		handler.mu.RUnlock()
	}
	return errors.Join(errs...)
}

// Handler returns the handler for a mock listener.
func (r *MockRegistry) Handler(listenerName string) http.Handler {
	if r == nil {
		return http.NotFoundHandler()
	}
	r.mu.RLock()
	handler := r.listeners[listenerName]
	r.mu.RUnlock()
	if handler == nil {
		return http.NotFoundHandler()
	}
	return handler
}

// RegisterAdminHandlers adds the admin API for managing mock scenarios.
func (r *MockRegistry) RegisterAdminHandlers(mux *http.ServeMux) {
	if r == nil || len(r.listeners) == 0 {
		return
	}
	mux.HandleFunc("GET /mocks/{listener}/scenarios", r.listScenarios)
	mux.HandleFunc("PUT /mocks/{listener}/scenarios/{scenario}", r.putScenario)
	mux.HandleFunc("DELETE /mocks/{listener}/scenarios/{scenario}", r.deleteScenario)
}

func (r *MockRegistry) listScenarios(w http.ResponseWriter, req *http.Request) {
	handler, ok := r.listener(req.PathValue("listener"))
	if !ok {
		http.NotFound(w, req)
		return
	}
	handler.mu.RLock()
	scenarios := make(map[string]config.MockScenario, len(handler.scenarios))
	for name, compiled := range handler.scenarios {
		scenarios[name] = cloneMockScenario(compiled.scenario)
	}
	handler.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scenarios); err != nil {
		http.Error(w, "failed to encode mock scenarios", http.StatusInternalServerError)
	}
}

func (r *MockRegistry) putScenario(w http.ResponseWriter, req *http.Request) {
	listenerName := req.PathValue("listener")
	scenarioName := req.PathValue("scenario")
	handler, ok := r.listener(listenerName)
	if !ok {
		http.NotFound(w, req)
		return
	}
	if !validMockScenarioName(scenarioName) {
		http.Error(w, "invalid scenario name", http.StatusBadRequest)
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, maxMockScenarioBody)
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	var scenario config.MockScenario
	if err := decoder.Decode(&scenario); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			http.Error(w, "mock scenario request body is too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, fmt.Sprintf("invalid mock scenario: %v", err), http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "request body must contain a single JSON object", http.StatusBadRequest)
		return
	}
	if err := config.ValidateMockScenario(scenario); err != nil {
		http.Error(w, fmt.Sprintf("invalid mock scenario: %v", err), http.StatusBadRequest)
		return
	}
	compiled, err := compileMockScenario(scenario)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid mock response code: %v", err), http.StatusBadRequest)
		return
	}

	handler.mu.Lock()
	handler.scenarios[scenarioName] = compiled
	handler.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (r *MockRegistry) deleteScenario(w http.ResponseWriter, req *http.Request) {
	handler, ok := r.listener(req.PathValue("listener"))
	if !ok {
		http.NotFound(w, req)
		return
	}

	handler.mu.Lock()
	scenarioName := req.PathValue("scenario")
	if _, exists := handler.scenarios[scenarioName]; !exists {
		handler.mu.Unlock()
		http.NotFound(w, req)
		return
	}
	delete(handler.scenarios, scenarioName)
	handler.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (r *MockRegistry) listener(name string) (*MockHandler, bool) {
	r.mu.RLock()
	handler, ok := r.listeners[name]
	r.mu.RUnlock()
	return handler, ok
}

func (h *MockHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	scenarioName := req.Header.Get(mockScenarioHeader)
	if scenarioName == "" {
		http.Error(w, "X-Mock-Scenario header is required", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	compiled, ok, ambiguous := h.selectScenario(scenarioName)
	compiled.scenario = cloneMockScenario(compiled.scenario)
	h.mu.RUnlock()
	if ambiguous {
		http.Error(w, "X-Mock-Scenario matches multiple scenarios", http.StatusBadRequest)
		return
	}
	pathValues, matches := mockRequestMatches(req, compiled.scenario.Match)
	if !ok || !matches {
		http.NotFound(w, req)
		return
	}
	for name, value := range pathValues {
		req.SetPathValue(name, value)
	}
	if compiled.err != nil {
		http.Error(w, fmt.Sprintf("mock response code failed to compile: %v", compiled.err), http.StatusInternalServerError)
		return
	}
	scenario := compiled.scenario

	var statusOverride *int
	if value, exists := req.Header[http.CanonicalHeaderKey(mockStatusHeader)]; exists {
		parsed, err := strconv.Atoi(strings.TrimSpace(strings.Join(value, ",")))
		if err != nil || parsed < 200 || parsed > 599 {
			http.Error(w, "X-Mock-Status must be between 200 and 599", http.StatusBadRequest)
			return
		}
		statusOverride = &parsed
	}
	var bodyOverride *string
	if value, exists := req.Header[http.CanonicalHeaderKey(mockBodyHeader)]; exists {
		parsed := strings.Join(value, ",")
		bodyOverride = &parsed
	}

	response := &http.Response{
		StatusCode: scenario.Response.Status,
		Header:     make(http.Header, len(scenario.Response.Headers)),
		Body:       io.NopCloser(strings.NewReader(scenario.Response.Body)),
		Request:    req,
	}
	if response.StatusCode == 0 {
		response.StatusCode = http.StatusOK
	}
	for name, value := range scenario.Response.Headers {
		response.Header.Set(name, value)
	}
	responseHeaderOverrides := make(http.Header)
	for name, values := range req.Header {
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(mockResponseHeaderPrefix)) {
			continue
		}
		responseHeader := name[len(mockResponseHeaderPrefix):]
		for _, value := range values {
			if err := config.ValidateMockScenario(config.MockScenario{
				Response: config.MockResponse{Headers: map[string]string{responseHeader: value}},
			}); err != nil {
				http.Error(w, "invalid X-Mock-Response-Header", http.StatusBadRequest)
				return
			}
		}
		responseHeaderOverrides.Del(responseHeader)
		for _, value := range values {
			responseHeaderOverrides.Add(responseHeader, value)
		}
	}
	if compiled.modifier != nil {
		if err := executeMockResponseCode(compiled.modifier, response); err != nil {
			if response.Body != nil {
				_ = response.Body.Close()
			}
			http.Error(w, fmt.Sprintf("mock response code failed: %v", err), http.StatusInternalServerError)
			return
		}
	}
	if response.Header == nil {
		response.Header = make(http.Header)
	}
	if response.StatusCode < 200 || response.StatusCode > 599 {
		closeMockResponseBody(response.Body)
		http.Error(w, "mock response code returned an invalid status", http.StatusInternalServerError)
		return
	}
	if err := validateGeneratedHeaders(response.Header); err != nil {
		closeMockResponseBody(response.Body)
		http.Error(w, fmt.Sprintf("mock response code returned invalid headers: %v", err), http.StatusInternalServerError)
		return
	}
	if statusOverride != nil {
		response.StatusCode = *statusOverride
	}
	for name, values := range responseHeaderOverrides {
		response.Header.Del(name)
		for _, value := range values {
			response.Header.Add(name, value)
		}
	}
	if response.Header.Get("Content-Type") == "" {
		response.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}

	var body []byte
	if response.Body != nil {
		var err error
		body, err = io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read mock response body: %v", err), http.StatusInternalServerError)
			return
		}
		if closeErr != nil {
			http.Error(w, fmt.Sprintf("failed to close mock response body: %v", closeErr), http.StatusInternalServerError)
			return
		}
	}
	if bodyOverride != nil {
		body = []byte(*bodyOverride)
	}
	for name, values := range response.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	if req.Method != http.MethodHead && response.StatusCode != http.StatusNoContent &&
		response.StatusCode != http.StatusResetContent && response.StatusCode != http.StatusNotModified {
		_, _ = w.Write(body)
	}
}

func (h *MockHandler) selectScenario(pattern string) (compiledMockScenario, bool, bool) {
	if scenario, ok := h.scenarios[pattern]; ok {
		return scenario, true, false
	}
	var selected compiledMockScenario
	matches := 0
	for name, scenario := range h.scenarios {
		if routing.MatchWildcard(pattern, name) {
			selected = scenario
			matches++
			if matches > 1 {
				return compiledMockScenario{}, false, true
			}
		}
	}
	return selected, matches == 1, false
}

func executeMockResponseCode(modifier func(*http.Response) error, response *http.Response) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic while executing response code: %v", recovered)
		}
	}()
	return modifier(response)
}

func closeMockResponseBody(body io.ReadCloser) {
	if body != nil {
		_ = body.Close()
	}
}

func compileMockScenario(scenario config.MockScenario) (compiledMockScenario, error) {
	if err := config.ValidateMockScenario(scenario); err != nil {
		return compiledMockScenario{}, err
	}
	compiled := compiledMockScenario{scenario: cloneMockScenario(scenario)}
	if scenario.Response.Code != "" {
		code, err := helper.ReadFile(scenario.Response.Code)
		if err != nil {
			compiled.err = err
			return compiled, err
		}
		scenario.Response.Code = string(code)
		modifier, err := dynamic.Response(scenario.Response.Code)
		if err != nil {
			compiled.err = err
			return compiled, err
		}
		compiled.modifier = modifier
	}
	return compiled, nil
}

func validateGeneratedHeaders(headers http.Header) error {
	values := make(map[string]string, len(headers))
	for name, headerValues := range headers {
		values[name] = strings.Join(headerValues, ",")
	}
	return config.ValidateMockScenario(config.MockScenario{
		Response: config.MockResponse{Headers: values},
	})
}

func mockRequestMatches(req *http.Request, match config.MockMatch) (map[string]string, bool) {
	if match.Method != "" && req.Method != match.Method {
		return nil, false
	}
	pathValues := make(map[string]string)
	if match.Path != "" {
		patternSegments := strings.Split(match.Path, "/")
		pathSegments := strings.Split(req.URL.Path, "/")
		if len(patternSegments) != len(pathSegments) {
			return nil, false
		}
		for i, patternSegment := range patternSegments {
			if strings.HasPrefix(patternSegment, ":") {
				if pathSegments[i] == "" {
					return nil, false
				}
				pathValues[patternSegment[1:]] = pathSegments[i]
			} else if patternSegment != pathSegments[i] {
				return nil, false
			}
		}
	}
	for name, value := range match.Headers {
		if req.Header.Get(name) != value {
			return nil, false
		}
	}
	return pathValues, true
}

func cloneMockScenario(scenario config.MockScenario) config.MockScenario {
	scenario.Match.Headers = cloneStringMap(scenario.Match.Headers)
	scenario.Response.Headers = cloneStringMap(scenario.Response.Headers)
	return scenario
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func validMockScenarioName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		strings.TrimSpace(name) == name && !strings.ContainsAny(name, "/\\?#\r\n")
}
