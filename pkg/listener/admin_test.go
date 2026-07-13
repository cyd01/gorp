package listener

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cyd01/gorp/pkg/backend"
	"github.com/cyd01/gorp/pkg/config"
)

func TestAdminBackendsEndpoint(t *testing.T) {
	availableBackend, err := backend.New("available", "http://127.0.0.1:8081", false, http.DefaultTransport)
	if err != nil {
		t.Fatalf("backend.New() error = %v", err)
	}
	unavailableBackend, err := backend.New("unavailable", "http://127.0.0.1:8082", false, http.DefaultTransport)
	if err != nil {
		t.Fatalf("backend.New() error = %v", err)
	}
	unavailableBackend.Healthy.Store(false)

	server, err := NewAdminServerWithBackends(":0", "", "", "", "", "", &config.Config{}, nil, []*backend.Backend{
		availableBackend,
		unavailableBackend,
	})
	if err != nil {
		t.Fatalf("NewAdminServerWithBackends() error = %v", err)
	}

	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backends", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /backends status = %d, want %d", response.Code, http.StatusOK)
	}

	var got []struct {
		Name        string `json:"name"`
		Destination string `json:"destination"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []struct {
		Name        string
		Destination string
		State       string
	}{
		{Name: "available", Destination: "http://127.0.0.1:8081", State: "closed"},
		{Name: "unavailable", Destination: "http://127.0.0.1:8082", State: "open"},
	}
	if len(got) != len(want) {
		t.Fatalf("backend count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Destination != want[i].Destination || got[i].State != want[i].State {
			t.Errorf("backend[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	response = httptest.NewRecorder()
	server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/backends", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /backends status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
