package backend

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// AWSSignerV4 implements AWS Signature Version 4 signing
// Used for S3, EC2, and other AWS services
type AWSSignerV4 struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Service         string
}

// SignRequest signs an HTTP request with AWS Signature Version 4
func (signer *AWSSignerV4) SignRequest(req *http.Request) error {
	// Set required headers
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	datestamp := now.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.Host)

	// Get request body hash
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		// Restore body for actual request
		req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	}

	hashedPayload := hashPayload(bodyBytes)
	req.Header.Set("X-Amz-Content-Sha256", hashedPayload)

	// Build canonical request
	canonicalRequest := buildCanonicalRequest(req, hashedPayload)

	// Build string to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request",
		datestamp, signer.Region, signer.Service)
	hashedCanonicalRequest := hashString(canonicalRequest)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, credentialScope, hashedCanonicalRequest)

	// Calculate signature
	kDate := hmacSha256([]byte("AWS4"+signer.SecretAccessKey), []byte(datestamp))
	kRegion := hmacSha256(kDate, []byte(signer.Region))
	kService := hmacSha256(kRegion, []byte(signer.Service))
	kSigning := hmacSha256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSha256(kSigning, []byte(stringToSign)))

	// Build authorization header
	signedHeaders := getSignedHeaders(req)
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		signer.AccessKeyID, credentialScope, signedHeaders, signature)

	req.Header.Set("Authorization", authHeader)

	return nil
}

// buildCanonicalRequest builds the canonical request for AWS Signature V4
func buildCanonicalRequest(req *http.Request, hashedPayload string) string {
	// Method
	canonical := req.Method + "\n"

	// Canonical URI
	canonical += req.URL.EscapedPath() + "\n"

	// Canonical Query String
	if req.URL.RawQuery != "" {
		// Parse and sort query parameters
		queryParams := parseQueryString(req.URL.RawQuery)
		canonical += queryParams + "\n"
	} else {
		canonical += "\n"
	}

	// Canonical Headers
	headers := getCanonicalHeaders(req)
	canonical += headers + "\n"

	// Signed Headers
	canonical += getSignedHeaders(req) + "\n"

	// Hashed Payload
	canonical += hashedPayload

	return canonical
}

// parseQueryString parses and canonicalizes query string
func parseQueryString(query string) string {
	params := make([]string, 0)
	pairs := strings.Split(query, "&")
	for _, pair := range pairs {
		if pair != "" {
			params = append(params, pair)
		}
	}
	sort.Strings(params)
	return strings.Join(params, "&")
}

// getCanonicalHeaders builds the canonical headers string
func getCanonicalHeaders(req *http.Request) string {
	headers := make(map[string]string)

	// Collect headers, converting to lowercase
	for key, values := range req.Header {
		lowKey := strings.ToLower(key)
		// Skip Authorization header in canonical request
		if lowKey == "authorization" {
			continue
		}
		// Join multiple values with comma
		headers[lowKey] = strings.Join(values, ",")
	}

	// Sort and format
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var canonical string
	for _, k := range keys {
		canonical += k + ":" + strings.TrimSpace(headers[k]) + "\n"
	}

	return canonical
}

// getSignedHeaders returns the list of signed headers
func getSignedHeaders(req *http.Request) string {
	headers := make(map[string]bool)

	for key := range req.Header {
		lowKey := strings.ToLower(key)
		// Skip authorization header
		if lowKey != "authorization" {
			headers[lowKey] = true
		}
	}

	// Ensure host and x-amz-* headers are included
	headers["host"] = true

	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return strings.Join(keys, ";")
}

// hashPayload returns the SHA256 hash of the payload
func hashPayload(payload []byte) string {
	return hashString(string(payload))
}

// hashString returns the SHA256 hash of a string
func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// hmacSha256 computes HMAC-SHA256
func hmacSha256(key, message []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(message)
	return h.Sum(nil)
}
