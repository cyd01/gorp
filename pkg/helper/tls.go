package helper

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	ocspx "golang.org/x/crypto/ocsp"
)

// Upstream: reverse proxy to backends.
func BuildTLSConfigForUpstream(insecure bool, servername, ca, cert, key string) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure,
		ServerName:         servername,
	}
	if ca != "" {
		data, err := ReadFile(ca)
		if err == nil {
			pool, err := x509.SystemCertPool()
			if err != nil {
				pool = x509.NewCertPool()
			}
			pool.AppendCertsFromPEM(
				data,
			)
			tlsConfig.RootCAs = pool
		}
	}
	if (cert != "") && (key != "") {
		certPEMBlock, err := ReadFile(cert)
		if err != nil {
			return tlsConfig, err
		}
		keyPEMBlock, err := ReadFile(key)
		if err != nil {
			return tlsConfig, err
		}
		certificates, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificates: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificates}
	}
	return tlsConfig, nil
}

func parseIssuerCertificateFromCAFile(caFile string, cert *x509.Certificate) (*x509.Certificate, error) {
	if caFile == "" {
		return nil, fmt.Errorf("no CA file configured")
	}
	data, err := ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	for len(data) > 0 {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		issuer, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		if issuer.IsCA && cert != nil && issuer.Subject.String() == cert.Issuer.String() {
			return issuer, nil
		}
	}
	return nil, fmt.Errorf("issuer certificate not found")
}

func fetchOCSPResponse(cert *x509.Certificate, issuer *x509.Certificate, responderURL string) ([]byte, error) {
	if responderURL == "" {
		if len(cert.OCSPServer) == 0 {
			return nil, fmt.Errorf("no OCSP responder URL available")
		}
		responderURL = cert.OCSPServer[0]
	}
	if responderURL == "" {
		return nil, fmt.Errorf("no OCSP responder URL configured")
	}
	request, err := ocspx.CreateRequest(cert, issuer, nil)
	if err != nil {
		return nil, fmt.Errorf("create OCSP request: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, responderURL, bytes.NewReader(request))
	if err != nil {
		return nil, fmt.Errorf("create OCSP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/ocsp-request")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("query OCSP responder: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("OCSP responder returned status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read OCSP response: %w", err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty OCSP response")
	}
	return body, nil
}

// Downstream: clients to reverse proxy.
func BuildTLSConfigForDownstream(cert, key, ca, crl, ocspURL string) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	// Load server certificates
	if (cert != "") && (key != "") {
		certPEMBlock, err := ReadFile(cert)
		if err != nil {
			return tlsConfig, err
		}
		keyPEMBlock, err := ReadFile(key)
		if err != nil {
			return tlsConfig, err
		}
		certificates, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificates: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificates}
	}

	if len(ca) > 0 {
		caPool := x509.NewCertPool()
		if caCert, err := ReadFile(ca); err == nil {
			caPool.AppendCertsFromPEM(caCert)
			tlsConfig.ClientCAs = caPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}

		if len(crl) > 0 {
			if data, err := ReadFile(crl); err == nil {
				block, _ := pem.Decode(data)
				if block != nil && block.Type == "X509 CRL" {
					var crlFile *x509.RevocationList
					if c, err := x509.ParseRevocationList(block.Bytes); err == nil {
						crlFile = c
					}
					tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
						if len(cs.PeerCertificates) == 0 {
							return fmt.Errorf("no client certificat")
						}
						certificate := cs.PeerCertificates[0]
						for _, revoked := range crlFile.RevokedCertificateEntries {
							if certificate.SerialNumber.Cmp(revoked.SerialNumber) == 0 {
								return fmt.Errorf("revoked certificat (serial number %s)", certificate.SerialNumber)
							}
						}
						return nil
					}
				}
			}
		} else {
			tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
				if len(cs.PeerCertificates) == 0 {
					return fmt.Errorf("no client certificat")
				}
				certificate := cs.PeerCertificates[0]
				issuer, err := parseIssuerCertificateFromCAFile(ca, certificate)
				if err != nil {
					return fmt.Errorf("resolve issuer certificate: %w", err)
				}
				effectiveURL := ocspURL
				if effectiveURL == "" && len(certificate.OCSPServer) > 0 {
					effectiveURL = certificate.OCSPServer[0]
				}
				if effectiveURL == "" {
					return nil
				}
				responseBytes, err := fetchOCSPResponse(certificate, issuer, effectiveURL)
				if err != nil {
					return fmt.Errorf("OCSP check failed: %w", err)
				}
				response, err := ocspx.ParseResponseForCert(responseBytes, certificate, issuer)
				if err != nil {
					return fmt.Errorf("parse OCSP response: %w", err)
				}
				switch response.Status {
				case ocspx.Good:
					return nil
				case ocspx.Revoked:
					return fmt.Errorf("revoked certificat by OCSP (serial number %s)", certificate.SerialNumber)
				default:
					return fmt.Errorf("certificate is not valid according to OCSP status %v", response.Status)
				}
			}
		}
	}
	return tlsConfig, nil
}

// downstream: clients -> reverse proxy
func BuildDynamicTLSConfig(caCertFile, caKeyFile, caKeyPassphrase string) (*tls.Config, error) {
	caCertPEM, err := ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate: %w", err)
	}
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil || caBlock.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("CA certificate file does not contain a certificate")
	}
	caCertificate, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}
	if !caCertificate.IsCA {
		return nil, fmt.Errorf("certificate is not a CA")
	}

	caKeyPEM, err := ReadFile(caKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read CA key: %w", err)
	}
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, fmt.Errorf("CA key file does not contain a PEM block")
	}
	if caKeyPassphrase != "" {
		if !x509.IsEncryptedPEMBlock(caKeyBlock) {
			return nil, fmt.Errorf("CA key is not an encrypted PEM block")
		}
		caKeyBlock.Bytes, err = x509.DecryptPEMBlock(caKeyBlock, []byte(caKeyPassphrase))
		if err != nil {
			return nil, fmt.Errorf("decrypt CA key: %w", err)
		}
	}
	caKey, err := parsePrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CA key: %w", err)
	}

	const maxCachedCertificates = 1024
	type cachedCertificate struct {
		certificate *tls.Certificate
		createdAt   time.Time
		expiresAt   time.Time
	}
	certCache := make(map[string]cachedCertificate)
	var cacheMu sync.RWMutex
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			host := strings.TrimSuffix(strings.ToLower(hello.ServerName), ".")
			if host == "" {
				return nil, fmt.Errorf("dynamic HTTPS requires SNI")
			}
			cacheMu.RLock()
			cached, found := certCache[host]
			cacheMu.RUnlock()
			if found && time.Now().Before(cached.expiresAt) {
				return cached.certificate, nil
			}
			if found {
				cacheMu.Lock()
				current, stillPresent := certCache[host]
				if stillPresent && current.certificate == cached.certificate && current.expiresAt.Equal(cached.expiresAt) {
					delete(certCache, host)
				}
				cacheMu.Unlock()
			}

			serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
			if err != nil {
				return nil, err
			}
			leaf := &x509.Certificate{
				SerialNumber: serial,
				Subject:      pkix.Name{CommonName: host},
				NotBefore:    time.Now().Add(-time.Minute),
				NotAfter:     time.Now().Add(24 * time.Hour),
				KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
				ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}
			if parsedIP := net.ParseIP(host); parsedIP != nil {
				leaf.IPAddresses = []net.IP{parsedIP}
			} else {
				leaf.DNSNames = []string{host}
			}
			privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return nil, err
			}
			der, err := x509.CreateCertificate(rand.Reader, leaf, caCertificate, &privateKey.PublicKey, caKey)
			if err != nil {
				return nil, fmt.Errorf("sign certificate for %q: %w", host, err)
			}
			certificate := &tls.Certificate{Certificate: [][]byte{der, caCertificate.Raw}, PrivateKey: privateKey, Leaf: leaf}
			cacheMu.Lock()
			if len(certCache) >= maxCachedCertificates {
				oldestHost := ""
				var oldestAt time.Time
				for cachedHost, cached := range certCache {
					if oldestHost == "" || cached.createdAt.Before(oldestAt) {
						oldestHost = cachedHost
						oldestAt = cached.createdAt
					}
				}
				delete(certCache, oldestHost)
			}
			certCache[host] = cachedCertificate{certificate: certificate, createdAt: time.Now(), expiresAt: leaf.NotAfter}
			cacheMu.Unlock()
			return certificate, nil
		},
	}, nil
}

func parsePrivateKey(data []byte) (any, error) {
	if key, err := x509.ParsePKCS8PrivateKey(data); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(data); err == nil {
		return key, nil
	}
	return x509.ParseECPrivateKey(data)
}
