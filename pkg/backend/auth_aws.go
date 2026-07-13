package backend

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	datestamp := now.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)

	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	hashedPayload := hashPayload(bodyBytes)
	req.Header.Set("X-Amz-Content-Sha256", hashedPayload)

	canonicalRequest, err := buildCanonicalRequest(req, hashedPayload)
	if err != nil {
		return err
	}

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request",
		datestamp, signer.Region, signer.Service)
	hashedCanonicalRequest := hashString(canonicalRequest)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, credentialScope, hashedCanonicalRequest)

	kDate := hmacSha256([]byte("AWS4"+signer.SecretAccessKey), []byte(datestamp))
	kRegion := hmacSha256(kDate, []byte(signer.Region))
	kService := hmacSha256(kRegion, []byte(signer.Service))
	kSigning := hmacSha256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSha256(kSigning, []byte(stringToSign)))

	signedHeaders := getSignedHeaders(req)

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		signer.AccessKeyID, credentialScope, signedHeaders, signature)

	req.Header.Set("Authorization", authHeader)
	return nil
}

// buildCanonicalRequest builds the canonical request for AWS Signature V4
func buildCanonicalRequest(req *http.Request, hashedPayload string) (string, error) {
	canonical := req.Method + "\n"
	canonical += req.URL.EscapedPath() + "\n"

	query, err := parseQueryString(req.URL.RawQuery)
	if err != nil {
		return "", fmt.Errorf("canonicalize AWS query string: %w", err)
	}
	canonical += query + "\n"
	canonical += getCanonicalHeaders(req) + "\n"
	canonical += getSignedHeaders(req) + "\n"
	canonical += hashedPayload
	return canonical, nil
}

type queryParameter struct {
	name  string
	value string
}

// parseQueryString parses and canonicalizes query string
func parseQueryString(query string) (string, error) {
	params := make([]queryParameter, 0)
	for _, pair := range strings.Split(query, "&") {
		if pair == "" {
			continue
		}
		name, value, _ := strings.Cut(pair, "=")
		decodedName, err := url.QueryUnescape(name)
		if err != nil {
			return "", err
		}
		decodedValue, err := url.QueryUnescape(value)
		if err != nil {
			return "", err
		}
		params = append(params, queryParameter{
			name:  awsURLEncode(decodedName),
			value: awsURLEncode(decodedValue),
		})
	}
	sort.Slice(params, func(i, j int) bool {
		if params[i].name == params[j].name {
			return params[i].value < params[j].value
		}
		return params[i].name < params[j].name
	})

	encoded := make([]string, 0, len(params))
	for _, param := range params {
		encoded = append(encoded, param.name+"="+param.value)
	}
	return strings.Join(encoded, "&"), nil
}

func awsURLEncode(value string) string {
	const hexDigits = "0123456789ABCDEF"
	var encoded strings.Builder
	for _, b := range []byte(value) {
		if (b >= 'A' && b <= 'Z') ||
			(b >= 'a' && b <= 'z') ||
			(b >= '0' && b <= '9') ||
			strings.ContainsRune("-_.~", rune(b)) {
			encoded.WriteByte(b)
			continue
		}
		encoded.WriteByte('%')
		encoded.WriteByte(hexDigits[b>>4])
		encoded.WriteByte(hexDigits[b&0x0f])
	}
	return encoded.String()
}

// getCanonicalHeaders builds the canonical headers string
func getCanonicalHeaders(req *http.Request) string {
	headers := make(map[string]string)
	headers["host"] = requestHost(req)

	for key, values := range req.Header {
		name := strings.ToLower(key)
		if !isSignedHeader(name) {
			continue
		}
		headers[name] = normalizeHeaderValue(strings.Join(values, ","))
	}

	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var canonical strings.Builder
	for _, key := range keys {
		canonical.WriteString(key)
		canonical.WriteByte(':')
		canonical.WriteString(headers[key])
		canonical.WriteByte('\n')
	}
	return canonical.String()
}

// getSignedHeaders returns the list of signed headers
func getSignedHeaders(req *http.Request) string {
	headers := make(map[string]bool)
	for key := range req.Header {
		name := strings.ToLower(key)
		if isSignedHeader(name) {
			headers[name] = true
		}
	}
	headers["host"] = true

	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ";")
}

func isSignedHeader(name string) bool {
	return name != "authorization" &&
		name != "host" &&
		name != "forwarded" &&
		!strings.HasPrefix(name, "x-forwarded-")
}

func normalizeHeaderValue(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func requestHost(req *http.Request) string {
	if req.Host != "" {
		return req.Host
	}
	if req.URL != nil {
		return req.URL.Host
	}
	return ""
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
