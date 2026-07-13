package listener

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestWithForwardedClientCertificate(t *testing.T) {
	certificate := &x509.Certificate{
		Raw:     []byte{1, 2, 3},
		Subject: pkix.Name{CommonName: "client.example"},
	}
	var got string
	handler := withForwardedClientCertificate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(forwardedClientCertHeader)
	}))
	request := httptest.NewRequest(http.MethodGet, "https://proxy.example/", nil)
	request.Header.Set(forwardedClientCertHeader, "untrusted")
	request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}}
	handler.ServeHTTP(httptest.NewRecorder(), request)
	decoded, err := url.QueryUnescape(got)
	if err != nil || got == "" || got == "untrusted" || !strings.Contains(decoded, "BEGIN CERTIFICATE") {
		t.Fatalf("forwarded certificate header = %q", got)
	}
	for _, field := range []string{"Hash=", "Cert=", "Chain=", "Subject=", "Issuer="} {
		if !strings.Contains(got, field) {
			t.Errorf("forwarded certificate header lacks %s: %q", field, got)
		}
	}
}
