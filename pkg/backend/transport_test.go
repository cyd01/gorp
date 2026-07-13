package backend

import (
	"net/http"
	"testing"
	"time"
)

func TestNewTransportUsesSpecificTimeouts(t *testing.T) {
	tr, err := NewTransport(TransportConfig{
		Timeout:         1 * time.Second,
		ConnectTimeout:  2 * time.Second,
		HeadersTimeout:  3 * time.Second,
		ResponseTimeout: 4 * time.Second,
		IdleTimeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}

	deadlineTr, ok := tr.(*deadlineTransport)
	if !ok {
		t.Fatalf("NewTransport() returned %T, want *deadlineTransport", tr)
	}
	if deadlineTr.timeout != 4*time.Second {
		t.Fatalf("deadline timeout = %v, want %v", deadlineTr.timeout, 4*time.Second)
	}
	baseTr, ok := deadlineTr.base.(*http.Transport)
	if !ok {
		t.Fatalf("wrapped transport = %T, want *http.Transport", deadlineTr.base)
	}
	if baseTr.ResponseHeaderTimeout != 3*time.Second {
		t.Fatalf("ResponseHeaderTimeout = %v, want %v", baseTr.ResponseHeaderTimeout, 3*time.Second)
	}
	if baseTr.IdleConnTimeout != 5*time.Second {
		t.Fatalf("IdleConnTimeout = %v, want %v", baseTr.IdleConnTimeout, 5*time.Second)
	}
	if baseTr.DialContext == nil {
		t.Fatal("DialContext is nil")
	}
}

func TestNewTransportCompressionSetting(t *testing.T) {
	tr, err := NewTransport(TransportConfig{
		Timeout:     5 * time.Second,
		Compression: true,
	})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}

	deadlineTr, ok := tr.(*deadlineTransport)
	if !ok {
		t.Fatalf("NewTransport() returned %T, want *deadlineTransport", tr)
	}
	baseTr, ok := deadlineTr.base.(*http.Transport)
	if !ok {
		t.Fatalf("wrapped transport = %T, want *http.Transport", deadlineTr.base)
	}
	if baseTr.DisableCompression {
		t.Fatal("DisableCompression = true, want false when compression is enabled")
	}

	tr, err = NewTransport(TransportConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	deadlineTr, ok = tr.(*deadlineTransport)
	if !ok {
		t.Fatalf("NewTransport() returned %T, want *deadlineTransport", tr)
	}
	baseTr, ok = deadlineTr.base.(*http.Transport)
	if !ok {
		t.Fatalf("wrapped transport = %T, want *http.Transport", deadlineTr.base)
	}
	if !baseTr.DisableCompression {
		t.Fatal("DisableCompression = false, want true when compression is disabled")
	}
}
