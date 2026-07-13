package routing

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitExtensions(t *testing.T) {
	called := 0
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called++
	})
	handler := NewBuiltinMiddleware().RateLimitExtensions(1, time.Minute, []string{"*.jpg", "*.png"})(next)

	request := func(path string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
		req.RemoteAddr = "192.0.2.10:1234"
		return req
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request("/images/photo.jpg"))
	if response.Code != http.StatusOK {
		t.Fatalf("first image status = %d, want %d", response.Code, http.StatusOK)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request("/images/other.jpg"))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("second image status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request("/videos/movie.mp4"))
	if response.Code != http.StatusOK {
		t.Fatalf("video status = %d, want %d", response.Code, http.StatusOK)
	}
	if called != 2 {
		t.Fatalf("next handler called %d times, want 2", called)
	}
}
