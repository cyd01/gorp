package helper

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ocspx "golang.org/x/crypto/ocsp"
)

func TestBuildDynamicTLSConfig(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	certFile := filepath.Join(directory, "ca.crt")
	keyFile := filepath.Join(directory, "ca.key")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}

	config, err := BuildDynamicTLSConfig(certFile, keyFile, "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := config.GetCertificate(&tls.ClientHelloInfo{ServerName: "Example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Leaf.VerifyHostname("example.test"); err != nil {
		t.Fatalf("generated certificate does not verify its hostname: %v", err)
	}
	second, err := config.GetCertificate(&tls.ClientHelloInfo{ServerName: "example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("expected certificates to be cached by hostname")
	}
}

func TestBuildTLSConfigForDownstreamCRLPrecedesOCSP(t *testing.T) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		SubjectKeyId:          []byte{1, 2, 3, 4, 5, 6, 7, 8},
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "client.example"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		AuthorityKeyId:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caTemplate, &certKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatal(err)
	}
	crlTemplate := &x509.RevocationList{
		Number:     big.NewInt(2),
		ThisUpdate: time.Now(),
		NextUpdate: time.Now().Add(time.Hour),
		RevokedCertificateEntries: []x509.RevocationListEntry{{
			SerialNumber:   cert.SerialNumber,
			RevocationTime: time.Now(),
		}},
	}
	crlDER, err := x509.CreateRevocationList(rand.Reader, crlTemplate, caTemplate, caKey)
	if err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()
	caFile := filepath.Join(directory, "ca.crt")
	crlFile := filepath.Join(directory, "ca.crl")
	ocspURL := "https://example.invalid/ocsp"
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(crlFile, pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: crlDER}), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := BuildTLSConfigForDownstream("", "", caFile, crlFile, ocspURL)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VerifyConnection == nil {
		t.Fatal("expected VerifyConnection to be configured")
	}

	err = cfg.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}})
	if err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("expected CRL revocation to win over OCSP; got %v", err)
	}
}

func TestBuildTLSConfigForDownstreamUsesCertificateOCSPFallback(t *testing.T) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		SubjectKeyId:          []byte{1, 2, 3, 4, 5, 6, 7, 8},
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "client.example"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		AuthorityKeyId:        []byte{1, 2, 3, 4, 5, 6, 7, 8},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caTemplate, &certKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	clientCert, err := x509.ParseCertificate(clientDER)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request")
		}
		response, err := ocspx.CreateResponse(caCert, caCert, ocspx.Response{
			SerialNumber: clientCert.SerialNumber,
			Status:       ocspx.Good,
			ThisUpdate:   time.Now(),
			NextUpdate:   time.Now().Add(time.Hour),
		}, caKey)
		if err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/ocsp-response")
		_, _ = w.Write(response)
	}))
	defer server.Close()
	clientCert.OCSPServer = []string{server.URL}

	responseBytes, err := fetchOCSPResponse(clientCert, caCert, "")
	if err != nil {
		t.Fatal(err)
	}
	response, err := ocspx.ParseResponseForCert(responseBytes, clientCert, caCert)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != ocspx.Good {
		t.Fatalf("expected OCSP GOOD, got %v", response.Status)
	}
}
