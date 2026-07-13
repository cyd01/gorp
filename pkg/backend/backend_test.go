package backend

import (
	"net/http"
	"testing"
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
