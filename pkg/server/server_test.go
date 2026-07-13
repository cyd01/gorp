package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConnectionLimiterRejectsWhenLimitReached(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	limited := newConnectionLimiter(handler, 1, "test-listener")

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodGet, "/", nil)
	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodGet, "/", nil)

	go func() {
		limited.ServeHTTP(first, firstReq)
	}()
	<-started

	result := make(chan int, 1)
	go func() {
		limited.ServeHTTP(second, secondReq)
		result <- second.Code
	}()

	select {
	case code := <-result:
		if code != http.StatusServiceUnavailable {
			t.Fatalf("expected second request to fail with 503, got %d", code)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("second request did not return promptly")
	}

	close(release)
}

func TestRequestBodyLimiterRejectsWhenLimitReached(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})
	limited := newRequestBodyLimiter(handler, 8)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("123456789"))
	res := httptest.NewRecorder()
	limited.ServeHTTP(res, req)
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", res.Code)
	}
}
