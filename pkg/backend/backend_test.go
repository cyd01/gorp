package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestBackendTryAcquireRespectsMaxConnections(t *testing.T) {
	b, err := New("test-backend", "http://example.com", false, http.DefaultTransport)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	b.MaxConnections = 1
	if !b.TryAcquire() {
		t.Fatal("expected first acquisition to succeed")
	}
	if b.TryAcquire() {
		t.Fatal("expected second acquisition to be rejected")
	}

	b.Release()
	if !b.TryAcquire() {
		t.Fatal("expected acquisition after release to succeed")
	}
	b.Release()
}

func TestPassiveCircuitBreakerCounts5xxOnceAndResetsAfterSuccess(t *testing.T) {
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		request := requests.Add(1)
		if request == 1 || request > 2 {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	b, err := New("passive-circuit-breaker", upstream.URL, false, http.DefaultTransport)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	serve := func() {
		recorder := httptest.NewRecorder()
		b.Proxy.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}

	serve()
	if got := b.failureCount.Load(); got != 1 {
		t.Fatalf("failure count after one 5xx response = %d, want 1", got)
	}

	serve()
	if got := b.failureCount.Load(); got != 0 {
		t.Fatalf("failure count after successful response = %d, want 0", got)
	}

	for i := 0; i < 4; i++ {
		serve()
	}
	if got := b.failureCount.Load(); got != 4 {
		t.Fatalf("failure count after four failures following success = %d, want 4", got)
	}
	if !b.Available() {
		t.Fatal("backend circuit opened before five failures since the last success")
	}
}

func TestPassiveCircuitBreakerAllowsOneHalfOpenProbe(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		wantHealthy bool
	}{
		{name: "failed probe reopens circuit", status: http.StatusServiceUnavailable},
		{name: "successful probe closes circuit", status: http.StatusOK, wantHealthy: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
			}))
			defer upstream.Close()

			b, err := New("half-open", upstream.URL, false, http.DefaultTransport)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			b.openCircuit()
			b.retryAfter.Store(time.Now().Add(-time.Second).UnixNano())

			if !b.Available() {
				t.Fatal("backend should allow one probe after the open interval")
			}
			if b.Available() {
				t.Fatal("backend should not allow a second concurrent probe")
			}

			b.Proxy.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
			if got := b.Available(); got != test.wantHealthy {
				t.Fatalf("Available() after probe = %t, want %t", got, test.wantHealthy)
			}
			if test.wantHealthy && b.breakerState.Load() != breakerClosed {
				t.Fatal("successful probe did not close the circuit")
			}
			if !test.wantHealthy && b.breakerState.Load() != breakerOpen {
				t.Fatal("failed probe did not reopen the circuit")
			}
			if !test.wantHealthy {
				remaining := time.Until(time.Unix(0, b.retryAfter.Load()))
				if remaining < 29*time.Second || remaining > 30*time.Second {
					t.Fatalf("circuit retry delay = %s, want about 30s", remaining)
				}
			}

		})
	}
}

func TestStateDoesNotTriggerHalfOpenProbe(t *testing.T) {
	b, err := New("state-probe", "http://127.0.0.1:8080", false, http.DefaultTransport)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	b.openCircuit()
	b.retryAfter.Store(time.Now().Add(-time.Second).UnixNano())

	if got := b.State(); got != "open" {
		t.Fatalf("State() = %q, want open", got)
	}
	if got := b.breakerState.Load(); got != breakerOpen {
		t.Fatalf("breaker state after State() = %d, want open", got)
	}
	if !b.Available() {
		t.Fatal("expected expired circuit to allow a half-open probe")
	}
	if got := b.State(); got != "half-open" {
		t.Fatalf("State() after probe acquisition = %q, want half-open", got)
	}
}

func TestActiveHealthCheckMarksBackendUnavailableAndRecovers(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		if !healthy.Load() {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	b, err := New("health-checked", server.URL, false, http.DefaultTransport)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	b.HealthCheckPath = "/health"
	b.HealthCheckTimeout = time.Second

	ctx := context.Background()
	b.checkHealth(ctx)
	if !b.Available() {
		t.Fatal("backend should be healthy after a successful check")
	}

	healthy.Store(false)
	b.checkHealth(ctx)
	if b.Available() {
		t.Fatal("backend should be unavailable after a failed check")
	}

	healthy.Store(true)
	b.checkHealth(ctx)
	if !b.Available() {
		t.Fatal("backend should recover after a successful check")
	}
}
