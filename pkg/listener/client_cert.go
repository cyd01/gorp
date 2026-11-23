package listener

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const forwardedClientCertHeader = "X-Forwarded-Client-Cert"

func withForwardedClientCertificate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del(forwardedClientCertHeader)
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			r.Header.Set(forwardedClientCertHeader, encodeClientCertificate(r.TLS.PeerCertificates))
		}
		next.ServeHTTP(w, r)
	})
}

func encodeClientCertificate(chain []*x509.Certificate) string {
	if len(chain) == 0 || chain[0] == nil {
		return ""
	}

	certificate := chain[0]
	hash := sha256.Sum256(certificate.Raw)
	parts := []string{
		"Hash=" + hex.EncodeToString(hash[:]),
		"Cert=" + url.QueryEscape(encodePEMCertificate(certificate)),
		"Chain=" + url.QueryEscape(encodePEMChain(chain)),
		"Subject=" + quoteXFCCAlways(certificate.Subject.String()),
		"Issuer=" + quoteXFCCAlways(certificate.Issuer.String()),
	}
	for _, uri := range certificate.URIs {
		parts = append(parts, "URI="+quoteXFCCValue(uri.String()))
	}
	for _, dns := range certificate.DNSNames {
		parts = append(parts, "DNS="+quoteXFCCValue(dns))
	}
	return strings.Join(parts, ";")
}

func encodePEMCertificate(certificate *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}))
}

func encodePEMChain(chain []*x509.Certificate) string {
	var builder strings.Builder
	for _, certificate := range chain {
		if certificate != nil {
			builder.WriteString(encodePEMCertificate(certificate))
		}
	}
	return builder.String()
}

func quoteXFCCValue(value string) string {
	if strings.ContainsAny(value, ",;=") || strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		return quoteXFCCAlways(value)
	}
	return value
}

func quoteXFCCAlways(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return strconv.Quote(value)
}
