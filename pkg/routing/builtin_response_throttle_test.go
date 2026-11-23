package routing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestThrottleResponseLimitsBodyReadRate(t *testing.T) {
	response := &http.Response{
		Body:   io.NopCloser(strings.NewReader(strings.Repeat("x", 20))),
		Header: make(http.Header),
	}
	modifier := (&BuiltinMiddleware{}).ThrottleResponse(100)
	if err := modifier(response); err != nil {
		t.Fatalf("ThrottleResponse() error = %v", err)
	}

	start := time.Now()
	body, err := io.ReadAll(response.Body)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(body) != 20 {
		t.Fatalf("body length = %d, want 20", len(body))
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("body read took %s, want at least 150ms", elapsed)
	}
}

func TestThrottleResponseStopsWaitingWhenRequestIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	response := &http.Response{
		Body:    io.NopCloser(strings.NewReader("x")),
		Request: httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx),
	}
	if err := (&BuiltinMiddleware{}).ThrottleResponse(1)(response); err != nil {
		t.Fatalf("ThrottleResponse() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := response.Body.Read(make([]byte, 1))
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Read() error = nil, want context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("Read() did not stop after request cancellation")
	}
}
