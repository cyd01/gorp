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

// MockRegistry stores response scenarios and request-first mocksrv routes.
type MockRegistry struct {
	mu        sync.RWMutex
	listeners map[string]*MockHandler
	mocksrv   map[string]*MockSrvHandler
}

// MockHandler serves responses from named scenarios without contacting an upstream.
type MockHandler struct {
	mu              sync.RWMutex
	scenarios       map[string]compiledMockScenario
	defaultScenario *compiledMockScenario
	errors          []error
}

type MockSrvHandler struct {
	mu     sync.RWMutex
	routes map[string]compiledMockSrvRoute
	errors []error
}

type compiledMockSrvRoute struct {
	route     config.MockSrvRoute
	scenarios map[string]compiledMockScenario
	fallback  *compiledMockScenario
}

type compiledMockScenario struct {
	scenario config.MockScenario
	modifier func(*http.Response) error
	err      error
}

// NewMockRegistry creates a registry initialized with the configured scenarios.
func NewMockRegistry(listeners []config.Listener) *MockRegistry {
	registry := &MockRegistry{
		listeners: make(map[string]*MockHandler),
		mocksrv:   make(map[string]*MockSrvHandler),
	}
	for _, l := range listeners {
		switch l.Type {
		case "mock":
			handler := &MockHandler{scenarios: make(map[string]compiledMockScenario)}
			if l.Mock != nil {
				if l.Mock.Default != nil {
					compiled, err := compileMockScenario(*l.Mock.Default)
					if err != nil {
						handler.errors = append(handler.errors, fmt.Errorf("listener %q default scenario: %w", l.Name, err))
					}
					handler.defaultScenario = &compiled
				}
				for name, scenario := range l.Mock.Scenarios {
					compiled, err := compileMockScenario(scenario)
					if err != nil {
						handler.errors = append(handler.errors, fmt.Errorf("listener %q scenario %q: %w", l.Name, name, err))
					}
					handler.scenarios[name] = compiled
				}
			}
			registry.listeners[l.Name] = handler
		case "mocksrv":
			handler := &MockSrvHandler{routes: make(map[string]compiledMockSrvRoute)}
			if l.MockSrv != nil {
				for name, route := range l.MockSrv.Routes {
					compiled, err := compileMockSrvRoute(route)
					if err != nil {
						handler.errors = append(handler.errors, fmt.Errorf("listener %q route %q: %w", l.Name, name, err))
					}
					handler.routes[name] = compiled
				}
			}
			registry.mocksrv[l.Name] = handler
		}
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
	for _, handler := range r.mocksrv {
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
	mocksrv := r.mocksrv[listenerName]
	r.mu.RUnlock()
	if mocksrv != nil {
		return mocksrv
	}
	if handler == nil {
		return http.NotFoundHandler()
	}
	return handler
}

// RegisterAdminHandlers adds the admin API for managing mock scenarios.
func (r *MockRegistry) RegisterAdminHandlers(mux *http.ServeMux) {
	if r == nil || len(r.listeners)+len(r.mocksrv) == 0 {
		return
	}
	mux.HandleFunc("GET /mocks/{listener}/scenarios", r.listScenarios)
	mux.HandleFunc("PUT /mocks/{listener}/scenarios/{scenario}", r.putScenario)
	mux.HandleFunc("DELETE /mocks/{listener}/scenarios/{scenario}", r.deleteScenario)
	mux.HandleFunc("GET /mocks/{listener}/routes", r.listMockSrvRoutes)
	mux.HandleFunc("PUT /mocks/{listener}/routes/{route}", r.putMockSrvRoute)
	mux.HandleFunc("DELETE /mocks/{listener}/routes/{route}", r.deleteMockSrvRoute)
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

func (r *MockRegistry) listMockSrvRoutes(w http.ResponseWriter, req *http.Request) {
	handler, ok := r.mockSrvListener(req.PathValue("listener"))
	if !ok {
		http.NotFound(w, req)
		return
	}
	handler.mu.RLock()
	routes := make(map[string]config.MockSrvRoute, len(handler.routes))
	for name, compiled := range handler.routes {
		routes[name] = cloneMockSrvRoute(compiled.route)
	}
	handler.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(routes); err != nil {
		http.Error(w, "failed to encode mocksrv routes", http.StatusInternalServerError)
	}
}

func (r *MockRegistry) putMockSrvRoute(w http.ResponseWriter, req *http.Request) {
	handler, ok := r.mockSrvListener(req.PathValue("listener"))
	if !ok {
		http.NotFound(w, req)
		return
	}
	routeName := req.PathValue("route")
	if !validMockScenarioName(routeName) {
		http.Error(w, "invalid route name", http.StatusBadRequest)
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, maxMockScenarioBody)
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	var route config.MockSrvRoute
	if err := decoder.Decode(&route); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			http.Error(w, "mocksrv route request body is too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, fmt.Sprintf("invalid mocksrv route: %v", err), http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "request body must contain a single JSON object", http.StatusBadRequest)
		return
	}
	if err := config.ValidateMockSrvRoute(route); err != nil {
		http.Error(w, fmt.Sprintf("invalid mocksrv route: %v", err), http.StatusBadRequest)
		return
	}
	compiled, err := compileMockSrvRoute(route)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid mocksrv response code: %v", err), http.StatusBadRequest)
		return
	}

	handler.mu.Lock()
	handler.routes[routeName] = compiled
	handler.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (r *MockRegistry) deleteMockSrvRoute(w http.ResponseWriter, req *http.Request) {
	handler, ok := r.mockSrvListener(req.PathValue("listener"))
	if !ok {
		http.NotFound(w, req)
		return
	}
	routeName := req.PathValue("route")
	handler.mu.Lock()
	if _, exists := handler.routes[routeName]; !exists {
		handler.mu.Unlock()
		http.NotFound(w, req)
		return
	}
	delete(handler.routes, routeName)
	handler.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (r *MockRegistry) listener(name string) (*MockHandler, bool) {
	r.mu.RLock()
	handler, ok := r.listeners[name]
	r.mu.RUnlock()
	return handler, ok
}

func (r *MockRegistry) mockSrvListener(name string) (*MockSrvHandler, bool) {
	r.mu.RLock()
	handler, ok := r.mocksrv[name]
	r.mu.RUnlock()
	return handler, ok
}

func (h *MockHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	scenarioName := req.Header.Get(mockScenarioHeader)

	h.mu.RLock()
	var fallback compiledMockScenario
	hasFallback := h.defaultScenario != nil
	if hasFallback {
		fallback = *h.defaultScenario
		fallback.scenario = cloneMockScenario(fallback.scenario)
	}
	var compiled compiledMockScenario
	var ok, ambiguous bool
	if scenarioName != "" {
		compiled, ok, ambiguous = h.selectScenario(scenarioName)
		compiled.scenario = cloneMockScenario(compiled.scenario)
	}
	h.mu.RUnlock()
	if ambiguous {
		http.Error(w, "X-Mock-Scenario matches multiple scenarios", http.StatusBadRequest)
		return
	}

	usingDefault := scenarioName == "" || !ok
	if scenarioName == "" {
		if !hasFallback {
			http.Error(w, "X-Mock-Scenario header is required", http.StatusBadRequest)
			return
		}
	} else if !ok {
		if !hasFallback {
			http.NotFound(w, req)
			return
		}
	}
	if usingDefault {
		compiled = fallback
	}

	pathValues, matches := mockRequestMatches(req, compiled.scenario.Match)
	if !matches && !usingDefault && hasFallback {
		compiled = fallback
		pathValues, matches = mockRequestMatches(req, compiled.scenario.Match)
	}
	if !matches {
		http.NotFound(w, req)
		return
	}
	for name, value := range pathValues {
		req.SetPathValue(name, value)
	}
	serveCompiledMockResponse(w, req, compiled)
}

func (h *MockSrvHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	scenarioName := req.Header.Get(mockScenarioHeader)

	h.mu.RLock()
	var selected compiledMockSrvRoute
	var selectedScore mockSrvMatchScore
	var pathValues map[string]string
	matches := 0
	for _, route := range h.routes {
		values, score, ok := mockSrvRequestMatches(req, route.route.Match)
		if !ok {
			continue
		}
		switch {
		case matches == 0 || score.moreSpecificThan(selectedScore):
			selected = route
			selectedScore = score
			pathValues = values
			matches = 1
		case score == selectedScore:
			matches++
		}
	}
	if matches > 1 {
		h.mu.RUnlock()
		http.Error(w, "request matches multiple equally specific mocksrv routes", http.StatusBadRequest)
		return
	}
	if matches == 0 {
		h.mu.RUnlock()
		http.NotFound(w, req)
		return
	}

	var compiled compiledMockScenario
	var found, ambiguous bool
	if scenarioName != "" {
		compiled, found, ambiguous = selectMockSrvResponse(selected.scenarios, scenarioName)
	}
	if ambiguous {
		h.mu.RUnlock()
		http.Error(w, "X-Mock-Scenario matches multiple scenarios", http.StatusBadRequest)
		return
	}
	if !found {
		if selected.fallback == nil {
			h.mu.RUnlock()
			if scenarioName == "" {
				http.Error(w, "X-Mock-Scenario header is required", http.StatusBadRequest)
			} else {
				http.NotFound(w, req)
			}
			return
		}
		compiled = *selected.fallback
	}
	h.mu.RUnlock()

	for name, value := range pathValues {
		req.SetPathValue(name, value)
	}
	serveCompiledMockResponse(w, req, compiled)
}

type mockSrvMatchScore struct {
	criteria int
	exact    int
	literals int
}

func (score mockSrvMatchScore) moreSpecificThan(other mockSrvMatchScore) bool {
	if score.criteria != other.criteria {
		return score.criteria > other.criteria
	}
	if score.exact != other.exact {
		return score.exact > other.exact
	}
	return score.literals > other.literals
}

func mockSrvRequestMatches(req *http.Request, match config.MockMatch) (map[string]string, mockSrvMatchScore, bool) {
	score := mockSrvMatchScore{}
	if match.Method != "" {
		score.criteria++
		score.exact++
		if req.Method != match.Method {
			return nil, mockSrvMatchScore{}, false
		}
	}
	pathValues := make(map[string]string)
	if match.Path != "" {
		score.criteria++
		if strings.Contains(match.Path, "**") {
			// Use MatchPath for multi-segment wildcard support
			res, ok := routing.MatchPath(match.Path, req.URL.Path)
			if !ok {
				return nil, mockSrvMatchScore{}, false
			}
			pathValues = res
			// Score calculation for MatchPath is complex; we'll approximate it
			// A very simple approximation:
			score.literals = len(match.Path) // Rough estimate
			return pathValues, score, true
		}

		pathSegments := strings.Split(match.Path, "/")
		requestSegments := strings.Split(req.URL.Path, "/")
		if len(pathSegments) != len(requestSegments) {
			return nil, mockSrvMatchScore{}, false
		}
		for i, pattern := range pathSegments {
			value := requestSegments[i]
			if strings.HasPrefix(pattern, ":") {
				if value == "" {
					return nil, mockSrvMatchScore{}, false
				}
				pathValues[pattern[1:]] = value
				continue
			}
			if !routing.MatchWildcard(pattern, value) {
				return nil, mockSrvMatchScore{}, false
			}
			score.literals += mockPatternLiteralCount(pattern)
		}
		if !hasMockWildcard(match.Path) && !hasMockPathParameter(match.Path) {
			score.exact++
		}
	}
	for name, pattern := range match.Headers {
		score.criteria++
		value := req.Header.Get(name)
		if !routing.MatchWildcard(pattern, value) {
			return nil, mockSrvMatchScore{}, false
		}
		if !hasMockWildcard(pattern) {
			score.exact++
		}
		score.literals += mockPatternLiteralCount(pattern)
	}
	return pathValues, score, true
}

func selectMockSrvResponse(scenarios map[string]compiledMockScenario, pattern string) (compiledMockScenario, bool, bool) {
	if scenario, ok := scenarios[pattern]; ok {
		return scenario, true, false
	}
	var selected compiledMockScenario
	matches := 0
	for name, scenario := range scenarios {
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

func hasMockWildcard(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func hasMockPathParameter(pattern string) bool {
	for _, segment := range strings.Split(pattern, "/") {
		if strings.HasPrefix(segment, ":") {
			return true
		}
	}
	return false
}

func mockPatternLiteralCount(pattern string) int {
	literals := 0
	inClass := false
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			if i+1 < len(pattern) {
				i++
				if !inClass {
					literals++
				}
			}
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '*', '?':
			if !inClass {
				continue
			}
			literals++
		default:
			if !inClass {
				literals++
			}
		}
	}
	return literals
}

func serveCompiledMockResponse(w http.ResponseWriter, req *http.Request, compiled compiledMockScenario) {
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
		modifier, err := compileMockResponseCode(scenario.Response.Code)
		if err != nil {
			compiled.err = err
			return compiled, err
		}
		compiled.modifier = modifier
	}
	return compiled, nil
}

func compileMockSrvRoute(route config.MockSrvRoute) (compiledMockSrvRoute, error) {
	if err := config.ValidateMockSrvRoute(route); err != nil {
		return compiledMockSrvRoute{}, err
	}
	compiled := compiledMockSrvRoute{
		route:     cloneMockSrvRoute(route),
		scenarios: make(map[string]compiledMockScenario, len(route.Scenarios)),
	}
	for name, response := range route.Scenarios {
		scenario, err := compileMockScenario(config.MockScenario{Response: response})
		if err != nil {
			return compiledMockSrvRoute{}, fmt.Errorf("scenario %q: %w", name, err)
		}
		compiled.scenarios[name] = scenario
	}
	if route.Default != nil {
		fallback, err := compileMockScenario(config.MockScenario{Response: *route.Default})
		if err != nil {
			return compiledMockSrvRoute{}, fmt.Errorf("default scenario: %w", err)
		}
		compiled.fallback = &fallback
	} else if response, ok := route.Scenarios["default"]; ok {
		fallback, err := compileMockScenario(config.MockScenario{Response: response})
		if err != nil {
			return compiledMockSrvRoute{}, fmt.Errorf("default scenario: %w", err)
		}
		compiled.fallback = &fallback
	}
	return compiled, nil
}

func compileMockResponseCode(source string) (func(*http.Response) error, error) {
	code, err := helper.ReadFile(source)
	if err != nil {
		return nil, err
	}
	return dynamic.Response(string(code))
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
		if strings.Contains(match.Path, "**") {
			res, ok := routing.MatchPath(match.Path, req.URL.Path)
			if !ok {
				return nil, false
			}
			pathValues = res
			return pathValues, true
		}

		patternSegments := strings.Split(match.Path, "/")
		pathSegments := strings.Split(req.URL.Path, "/")
		if len(patternSegments) != len(pathSegments) {
			return nil, false
		}
		for i, patternSegment := range patternSegments {
			value := pathSegments[i]
			if strings.HasPrefix(patternSegment, ":") {
				if value == "" {
					return nil, false
				}
				pathValues[patternSegment[1:]] = value
				continue
			}
			if !routing.MatchWildcard(patternSegment, value) {
				return nil, false
			}
		}
	}
	for name, pattern := range match.Headers {
		value := req.Header.Get(name)
		if !routing.MatchWildcard(pattern, value) {
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

func cloneMockSrvRoute(route config.MockSrvRoute) config.MockSrvRoute {
	route.Match.Headers = cloneStringMap(route.Match.Headers)
	if route.Scenarios != nil {
		scenarios := make(map[string]config.MockResponse, len(route.Scenarios))
		for name, response := range route.Scenarios {
			response.Headers = cloneStringMap(response.Headers)
			scenarios[name] = response
		}
		route.Scenarios = scenarios
	}
	if route.Default != nil {
		fallback := *route.Default
		fallback.Headers = cloneStringMap(fallback.Headers)
		route.Default = &fallback
	}
	return route
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
