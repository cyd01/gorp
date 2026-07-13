package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadinessHandlerTracksState(t *testing.T) {
	readiness := NewReadiness()

	assertResponse := func(wantStatus int, wantBody string, wantContentType string) {
		t.Helper()
		response := httptest.NewRecorder()
		readiness.Handler(response, httptest.NewRequest(http.MethodGet, "/ready", nil))
		if response.Code != wantStatus {
			t.Errorf("status = %d, want %d", response.Code, wantStatus)
		}
		if response.Body.String() != wantBody {
			t.Errorf("body = %q, want %q", response.Body.String(), wantBody)
		}
		if got := response.Header().Get("Content-Type"); got != wantContentType {
			t.Errorf("Content-Type = %q, want %q", got, wantContentType)
		}
	}

	assertResponse(http.StatusServiceUnavailable, "Not Ready\n\n", "text/plain; charset=utf-8")
	readiness.SetReady(true)
	assertResponse(http.StatusOK, "OK\n", "text/plain; charset=utf-8")
	readiness.SetReady(false)
	assertResponse(http.StatusServiceUnavailable, "Not Ready\n\n", "text/plain; charset=utf-8")
}
