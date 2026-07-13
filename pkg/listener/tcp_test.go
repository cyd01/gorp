package listener

import (
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
