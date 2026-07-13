package backend

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestNewTransportDefaultsAndBoundsRetries(t *testing.T) {
	transport, err := NewTransport(TransportConfig{Retries: 1})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	retry, ok := transport.(*retryTransport)
	if !ok {
		t.Fatalf("transport type = %T, want *retryTransport", transport)
	}
	if retry.backoff != 100*time.Millisecond {
		t.Fatalf("retry backoff = %s, want 100ms", retry.backoff)
	}

	if _, err := NewTransport(TransportConfig{Retries: 6}); err == nil {
		t.Fatal("NewTransport() accepted more than five retries")
	}
}

func TestRetryTransportRetriesGETAndHEAD(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			var attempts atomic.Int32
			transport := &retryTransport{
				retries: 1,
				base: roundTripFunc(func(*http.Request) (*http.Response, error) {
					if attempts.Add(1) == 1 {
						return &http.Response{
							StatusCode: http.StatusServiceUnavailable,
							Body:       io.NopCloser(strings.NewReader("unavailable")),
							Header:     make(http.Header),
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       http.NoBody,
						Header:     make(http.Header),
					}, nil
				}),
			}
			request, err := http.NewRequest(method, "http://example.test/", nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := transport.RoundTrip(request)
			if err != nil {
				t.Fatalf("RoundTrip() error = %v", err)
			}
			if response.StatusCode != http.StatusOK || attempts.Load() != 2 {
				t.Fatalf("status/attempts = %d/%d, want 200/2", response.StatusCode, attempts.Load())
			}
		})
	}
}

func TestRetryTransportDoesNotRetryUnsafeOrUnreplayableRequests(t *testing.T) {
	for _, test := range []struct {
		name   string
		method string
		body   bool
	}{
		{name: "POST", method: http.MethodPost},
		{name: "GET with streaming body", method: http.MethodGet, body: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var attempts atomic.Int32
			transport := &retryTransport{
				retries: 3,
				base: roundTripFunc(func(*http.Request) (*http.Response, error) {
					attempts.Add(1)
					return &http.Response{
						StatusCode: http.StatusServiceUnavailable,
						Body:       io.NopCloser(strings.NewReader("unavailable")),
						Header:     make(http.Header),
					}, nil
				}),
			}
			var body io.Reader
			if test.body {
				body = strings.NewReader("not replayable")
			}
			request, err := http.NewRequest(test.method, "http://example.test/", body)
			if err != nil {
				t.Fatal(err)
			}
			if test.body {
				request.GetBody = nil
			}
			_, err = transport.RoundTrip(request)
			if err != nil {
				t.Fatalf("RoundTrip() error = %v", err)
			}
			if got := attempts.Load(); got != 1 {
				t.Fatalf("attempts = %d, want 1", got)
			}
		})
	}
}

func TestRetryTransportReplaysOnlyUsingGetBody(t *testing.T) {
	var attempts atomic.Int32
	transport := &retryTransport{
		retries: 1,
		base: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read attempt body: %v", err)
			}
			if string(body) != "replayable" {
				t.Fatalf("attempt body = %q", body)
			}
			if attempts.Add(1) == 1 {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(strings.NewReader("retry")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
		}),
	}
	request, err := http.NewRequest(http.MethodGet, "http://example.test/", strings.NewReader("replayable"))
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	if response.StatusCode != http.StatusOK || attempts.Load() != 2 {
		t.Fatalf("status/attempts = %d/%d, want 200/2", response.StatusCode, attempts.Load())
	}
}

func TestRetryTransportStopsWhenRequestContextIsCancelled(t *testing.T) {
	transport := &retryTransport{
		retries: 2,
		backoff: time.Hour,
		base: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader("unavailable")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	request, err := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)
	cancel()

	_, err = transport.RoundTrip(request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RoundTrip() error = %v, want context.Canceled", err)
	}
}
