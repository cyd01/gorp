package dynamic

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequest(t *testing.T) {
	middleware, err := Request(`package dynamic
import "net/http"
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Dynamic", "yes")
		next.ServeHTTP(w, r)
	})
}`)
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Dynamic"); got != "yes" {
			t.Errorf("X-Dynamic = %q, want yes", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	recording := httptest.NewRecorder()
	handler.ServeHTTP(recording, httptest.NewRequest(http.MethodGet, "/", nil))
	if recording.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recording.Code, http.StatusNoContent)
	}
}

func TestResponse(t *testing.T) {
	modifier, err := Response(`package dynamic
import "net/http"
func Middleware(response *http.Response) error {
	response.Header.Set("X-Dynamic", "yes")
	return nil
}`)
	if err != nil {
		t.Fatalf("Response() error = %v", err)
	}

	response := &http.Response{Header: make(http.Header)}
	if err := modifier(response); err != nil {
		t.Fatalf("modifier() error = %v", err)
	}
	if got := response.Header.Get("X-Dynamic"); got != "yes" {
		t.Errorf("X-Dynamic = %q, want yes", got)
	}
}
