package backend

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestBuildCanonicalRequestUsesUpstreamHostAndSkipsForwardingHeaders(t *testing.T) {
	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Scheme: "https", Host: "bucket.s3.eu-west-3.amazonaws.com", Path: "/object"},
		Header: http.Header{
			"X-Forwarded-For":  {"192.0.2.10"},
			"X-Forwarded-Host": {"gorp.example.test"},
			"X-Amz-Date":       {"20261019T165419Z"},
		},
	}
	const payloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	canonical, err := buildCanonicalRequest(req, payloadHash)
	if err != nil {
		t.Fatalf("buildCanonicalRequest() error = %v", err)
	}

	if !strings.Contains(canonical, "\nhost:bucket.s3.eu-west-3.amazonaws.com\nx-amz-date:20261019T165419Z\n") {
		t.Fatalf("canonical request has wrong headers:\n%s", canonical)
	}
	if strings.Contains(canonical, "x-forwarded-") {
		t.Fatalf("canonical request contains forwarding headers:\n%s", canonical)
	}
	if got, want := getSignedHeaders(req), "host;x-amz-date"; got != want {
		t.Fatalf("getSignedHeaders() = %q, want %q", got, want)
	}
}

func TestBuildCanonicalRequestUsesExplicitRequestHost(t *testing.T) {
	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Scheme: "https", Host: "upstream.example.test", Path: "/"},
		Host:   "bucket.s3.eu-west-3.amazonaws.com",
		Header: make(http.Header),
	}

	if got, want := getCanonicalHeaders(req), "host:bucket.s3.eu-west-3.amazonaws.com\n"; got != want {
		t.Fatalf("getCanonicalHeaders() = %q, want %q", got, want)
	}
}

func TestParseQueryStringCanonicalizesEncodedParameters(t *testing.T) {
	got, err := parseQueryString("prefix=a+b&uploads&z=%2F&prefix=a%20c")
	if err != nil {
		t.Fatalf("parseQueryString() error = %v", err)
	}
	const want = "prefix=a%20b&prefix=a%20c&uploads=&z=%2F"
	if got != want {
		t.Fatalf("parseQueryString() = %q, want %q", got, want)
	}
}

func TestParseQueryStringRejectsMalformedEscapes(t *testing.T) {
	if _, err := parseQueryString("key=%zz"); err == nil {
		t.Fatal("parseQueryString() error = nil, want malformed escape error")
	}
}

func TestSignRequestPreservesBodyAndDoesNotSetHeaderHost(t *testing.T) {
	req := &http.Request{
		Method: "PUT",
		URL:    &url.URL{Scheme: "https", Host: "s3.example.test", Path: "/bucket/object"},
		Header: make(http.Header),
		Body:   io.NopCloser(strings.NewReader("payload")),
	}
	signer := &AWSSignerV4{
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		Region:          "eu-west-3",
		Service:         "s3",
	}

	if err := signer.SignRequest(req); err != nil {
		t.Fatalf("SignRequest() error = %v", err)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("reading restored request body: %v", err)
	}
	if string(body) != "payload" {
		t.Fatalf("request body = %q, want %q", body, "payload")
	}
	if _, exists := req.Header["Host"]; exists {
		t.Fatal("SignRequest() set Host in request headers; it must use Request.Host or URL.Host")
	}
	if !strings.Contains(req.Header.Get("Authorization"), "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Fatalf("Authorization header has unexpected signed headers: %s", req.Header.Get("Authorization"))
	}
}
