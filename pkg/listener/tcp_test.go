package listener

import (
	"io"
	"net"
	"testing"

	"github.com/cyd01/gorp/pkg/config"
)

func TestSelectTCPBackends(t *testing.T) {
	fallback := []config.Backend{{Name: "fallback"}}
	database := []config.Backend{{Name: "database"}}
	routes := []config.TCPRoute{
		{Name: "database-route", Hosts: []string{"db.example.com", "*.db.example.com"}, Backends: database},
	}

	if got := selectTCPBackends("db.example.com", fallback, routes); got[0].Name != "database" {
		t.Fatalf("exact SNI match selected %q, want database", got[0].Name)
	}
	if got := selectTCPBackends("read.db.example.com", fallback, routes); got[0].Name != "database" {
		t.Fatalf("wildcard SNI match selected %q, want database", got[0].Name)
	}
	if got := selectTCPBackends("other.example.com", fallback, routes); got[0].Name != "fallback" {
		t.Fatalf("unmatched SNI selected %q, want fallback", got[0].Name)
	}
}

func TestValidateInitialBytesReplaysStream(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_, _ = client.Write([]byte{16, 42, 4, 'p', 'a', 'y', 'l', 'o', 'a', 'd'})
		_ = client.Close()
	}()

	checked, err := validateInitialBytes(server, []config.TCPByteCheck{
		{Values: []byte{16}},
		{Any: true},
		{Values: []byte{3, 4, 5}},
	})
	if err != nil {
		t.Fatalf("validateInitialBytes() error = %v", err)
	}
	got, err := io.ReadAll(checked)
	if err != nil {
		t.Fatalf("read replayed stream: %v", err)
	}
	want := []byte{16, 42, 4, 'p', 'a', 'y', 'l', 'o', 'a', 'd'}
	if string(got) != string(want) {
		t.Fatalf("replayed stream = %v, want %v", got, want)
	}
}

func TestValidateInitialBytesRejectsValue(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_, _ = client.Write([]byte{16, 42, 2})
		_ = client.Close()
	}()

	if _, err := validateInitialBytes(server, []config.TCPByteCheck{
		{Values: []byte{16}},
		{Any: true},
		{Values: []byte{3, 4, 5}},
	}); err == nil {
		t.Fatal("validateInitialBytes() error = nil, want rejected byte")
	}
}

func TestSelectTCPRoute(t *testing.T) {
	routes := []config.TCPRoute{
		{Name: "route-a", Hosts: []string{"api.example.com"}, TLS: &config.TLSConfig{CertFile: "cert-a.crt", KeyFile: "cert-a.key"}, Backends: []config.Backend{{Name: "backend-a"}}},
		{Name: "route-b", Hosts: []string{"*.example.com"}, TLS: &config.TLSConfig{CertFile: "cert-b.crt", KeyFile: "cert-b.key"}, Backends: []config.Backend{{Name: "backend-b"}}},
	}

	if got := selectTCPRoute("api.example.com", routes); got == nil || got.Name != "route-a" {
		t.Fatalf("exact SNI route selected %v, want route-a", got)
	}
	if got := selectTCPRoute("foo.example.com", routes); got == nil || got.Name != "route-b" {
		t.Fatalf("wildcard SNI route selected %v, want route-b", got)
	}
	if got := selectTCPRoute("other.domain", routes); got != nil {
		t.Fatalf("unmatched SNI route selected %v, want nil", got)
	}
}
