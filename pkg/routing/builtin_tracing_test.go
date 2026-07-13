package routing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDistributedTracingCreatesSpan(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://example.test", nil)
	response := httptest.NewRecorder()
	var propagated string

	handler := NewBuiltinMiddleware().DistributedTracing()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		propagated = r.Header.Get(traceParentHeader)
	}))
	handler.ServeHTTP(response, request)

	traceID, spanID, flags, ok := parseTraceParent(propagated)
	if !ok || traceID == "" || spanID == "" || flags != "00" {
		t.Fatalf("generated invalid traceparent %q", propagated)
	}
	if response.Header().Get(traceParentHeader) != propagated {
		t.Fatalf("response traceparent = %q, want %q", response.Header().Get(traceParentHeader), propagated)
	}
}

func TestDistributedTracingCreatesChildSpan(t *testing.T) {
	const incoming = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	request := httptest.NewRequest(http.MethodGet, "http://example.test", nil)
	request.Header.Set(traceParentHeader, incoming)
	response := httptest.NewRecorder()
	var propagated string

	handler := NewBuiltinMiddleware().DistributedTracing()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		propagated = r.Header.Get(traceParentHeader)
	}))
	handler.ServeHTTP(response, request)

	parts := strings.Split(propagated, "-")
	if len(parts) != 4 {
		t.Fatalf("generated invalid traceparent %q", propagated)
	}
	if parts[1] != strings.Split(incoming, "-")[1] {
		t.Fatalf("trace ID = %q, want %q", parts[1], strings.Split(incoming, "-")[1])
	}
	if parts[2] == strings.Split(incoming, "-")[2] {
		t.Fatalf("proxy reused incoming span ID %q", parts[2])
	}
	if parts[3] != "01" {
		t.Fatalf("trace flags = %q, want 01", parts[3])
	}
}

func TestParseTraceParentRejectsInvalidValue(t *testing.T) {
	if _, _, _, ok := parseTraceParent("not-a-traceparent"); ok {
		t.Fatal("invalid traceparent was accepted")
	}
}
