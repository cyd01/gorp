package selector

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cyd01/gorp/pkg/backend"
	"github.com/cyd01/gorp/pkg/config"
)

func testBackends(health ...bool) []*backend.Backend {
	backends := make([]*backend.Backend, len(health))
	for i, healthy := range health {
		backends[i] = &backend.Backend{Name: string(rune('a' + i))}
		backends[i].Healthy.Store(healthy)
	}
	return backends
}

func TestFirstAliveSelectsFirstAvailableBackend(t *testing.T) {
	backends := testBackends(false, true, true)
	selected, err := NewFirstAlive(backends).Select(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selected != backends[1] {
		t.Fatalf("Select() = %q, want %q", selected.Name, backends[1].Name)
	}
}

func TestRoundRobinSelectsAvailableBackendsInOrder(t *testing.T) {
	backends := testBackends(true, false, true)
	selector := NewRoundRobin(backends)
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	for _, want := range []*backend.Backend{backends[0], backends[2], backends[0]} {
		selected, err := selector.Select(request)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		if selected != want {
			t.Fatalf("Select() = %q, want %q", selected.Name, want.Name)
		}
	}
}

func TestSelectorsReportMissingAndUnavailableBackends(t *testing.T) {
	tests := []struct {
		name string
		new  func([]*backend.Backend) Selector
	}{
		{name: "first alive", new: func(b []*backend.Backend) Selector { return NewFirstAlive(b) }},
		{name: "round robin", new: func(b []*backend.Backend) Selector { return NewRoundRobin(b) }},
		{name: "least connections", new: func(b []*backend.Backend) Selector { return NewLeastConnections(b) }},
		{name: "power of two", new: func(b []*backend.Backend) Selector { return NewPowerOfTwoChoices(b) }},
		{name: "random", new: func(b []*backend.Backend) Selector { return NewRandomSelector(b) }},
		{name: "affinity", new: func(b []*backend.Backend) Selector { return NewHeaderSelector(b, "X-Key") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if _, err := test.new(nil).Select(request); err != ErrNoBackend {
				t.Fatalf("Select() error = %v, want %v", err, ErrNoBackend)
			}
			if _, err := test.new(testBackends(false, false)).Select(request); err != ErrNoValidBackend {
				t.Fatalf("Select() error = %v, want %v", err, ErrNoValidBackend)
			}
		})
	}
}

func TestRandomSelectorExcludesUnavailableBackends(t *testing.T) {
	backends := testBackends(false, true, false)
	selector := NewRandomSelector(backends)
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	for range 50 {
		selected, err := selector.Select(request)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		if selected != backends[1] {
			t.Fatalf("Select() = %q, want only available backend %q", selected.Name, backends[1].Name)
		}
	}
}

func TestLeastConnectionsSelectsLeastLoadedAvailableBackend(t *testing.T) {
	backends := testBackends(true, true, false)
	backends[0].ActiveConnections.Store(5)
	backends[1].ActiveConnections.Store(2)

	selected, err := NewLeastConnections(backends).Select(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selected != backends[1] {
		t.Fatalf("Select() = %q, want %q", selected.Name, backends[1].Name)
	}
}

func TestPowerOfTwoChoicesSelectsLowerLoad(t *testing.T) {
	backends := testBackends(true, true)
	backends[0].ActiveConnections.Store(1)
	backends[1].ActiveConnections.Store(3)

	for range 20 {
		selected, err := NewPowerOfTwoChoices(backends).Select(httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		if selected != backends[0] {
			t.Fatalf("Select() = %q, want lower-load backend %q", selected.Name, backends[0].Name)
		}
	}
}

func TestAffinitySelectorsUseTheirConfiguredRequestKey(t *testing.T) {
	backends := testBackends(true, true, true, true, true, true, true)
	tests := []struct {
		name     string
		selector *AffinitySelector
		request  *http.Request
		key      string
	}{
		{
			name:     "cookie",
			selector: NewCookieSelector(backends, "session"),
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			key:      "cookie-key",
		},
		{
			name:     "header",
			selector: NewHeaderSelector(backends, "X-Tenant"),
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			key:      "header-key",
		},
		{
			name:     "query parameter",
			selector: NewQueryParamSelector(backends, "tenant"),
			request:  httptest.NewRequest(http.MethodGet, "/?tenant=query-key", nil),
			key:      "query-key",
		},
		{
			name:     "source ip",
			selector: NewIPSelector(backends),
			request:  httptest.NewRequest(http.MethodGet, "/", nil),
			key:      "192.0.2.17",
		},
	}
	tests[0].request.AddCookie(&http.Cookie{Name: "session", Value: tests[0].key})
	tests[1].request.Header.Set("X-Tenant", tests[1].key)
	tests[3].request.RemoteAddr = "192.0.2.17:443"

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selected, err := test.selector.Select(test.request)
			if err != nil {
				t.Fatalf("Select() error = %v", err)
			}
			want := backends[hashIndex(test.key, len(backends))]
			if selected != want {
				t.Fatalf("Select() = %q, want %q", selected.Name, want.Name)
			}
		})
	}
}

func TestFactoryQuerySelectorReadsQueryParameter(t *testing.T) {
	backends := testBackends(true, true, true, true, true, true, true)
	const queryValue = "tenant-from-query"
	cookieValue := "tenant-from-cookie"
	for hashIndex(queryValue, len(backends)) == hashIndex(cookieValue, len(backends)) {
		cookieValue += "-different"
	}

	request := httptest.NewRequest(http.MethodGet, "/?tenant="+queryValue, nil)
	request.AddCookie(&http.Cookie{Name: "tenant", Value: cookieValue})
	selector := New(config.SelectorConfig{
		Type:   "query",
		Config: map[string]interface{}{"name": "tenant"},
	}, backends)

	selected, err := selector.Select(request)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	want := backends[hashIndex(queryValue, len(backends))]
	if selected != want {
		t.Fatalf("Select() = %q, want query-affinity backend %q", selected.Name, want.Name)
	}
}

func TestHeaderSelectorRoutesBackendAffinityKey(t *testing.T) {
	backends := testBackends(true, true, true)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Affinity", BackendAffinityKey(2))

	selected, err := NewHeaderSelector(backends, "X-Affinity").Select(request)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selected != backends[2] {
		t.Fatalf("Select() = %q, want backend %q", selected.Name, backends[2].Name)
	}
}

func TestAffinitySelectorFallsBackWhenPinnedBackendIsUnavailable(t *testing.T) {
	backends := testBackends(false, true, true)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Affinity", BackendAffinityKey(0))

	selected, err := NewHeaderSelector(backends, "X-Affinity").Select(request)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selected != backends[1] {
		t.Fatalf("Select() = %q, want next available backend %q", selected.Name, backends[1].Name)
	}
}
