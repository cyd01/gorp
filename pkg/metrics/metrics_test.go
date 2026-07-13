package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandlerExportsProxyMetrics(t *testing.T) {
	ObserveRequest("test-backend", "GET", 200, 150*time.Millisecond)
	ObserveBackendError("test-backend")
	SetActiveConnections("test-backend", 2)

	recording := httptest.NewRecorder()
	Handler().ServeHTTP(recording, httptest.NewRequest("GET", "/metrics", nil))

	body := recording.Body.String()
	for _, metric := range []string{
		`gorp_proxy_requests_total{backend="test-backend",method="GET",status="200"}`,
		`gorp_proxy_backend_errors_total{backend="test-backend"}`,
		`gorp_proxy_active_connections{backend="test-backend"} 2`,
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("metrics output does not contain %q", metric)
		}
	}
}

func TestInstrumentHandlerExportsListenerMetrics(t *testing.T) {
	handler := InstrumentHandler("test-listener", "http", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	recording := httptest.NewRecorder()
	handler.ServeHTTP(recording, httptest.NewRequest("POST", "/", nil))

	body := HandlerOutput()
	for _, metric := range []string{
		`gorp_listener_requests_total{listener="test-listener",method="POST",status="201",type="http"}`,
		`gorp_listener_active_connections{listener="test-listener",type="http"}`,
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("metrics output does not contain %q", metric)
		}
	}
}

func HandlerOutput() string {
	recording := httptest.NewRecorder()
	Handler().ServeHTTP(recording, httptest.NewRequest("GET", "/metrics", nil))
	return recording.Body.String()
}
