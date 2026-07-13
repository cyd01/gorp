package routing

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type trackingBody struct {
	reader io.Reader
}

type informationalResponseWriter struct {
	header http.Header
	status int
	codes  []int
	body   bytes.Buffer
}

func (w *informationalResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *informationalResponseWriter) WriteHeader(status int) {
	w.codes = append(w.codes, status)
	if status >= 200 || status == http.StatusSwitchingProtocols {
		w.status = status
	}
}

func (w *informationalResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(body)
}

func (body *trackingBody) Read(buffer []byte) (int, error) {
	return body.reader.Read(buffer)
}

func (*trackingBody) Close() error {
	return nil
}

func TestLoggingCapturesStatusAndRouteWithoutConsumingBody(t *testing.T) {
	var output bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	}()

	body := &trackingBody{reader: strings.NewReader("streamed request body")}
	request := httptest.NewRequest(http.MethodPost, "/upload", nil)
	request.Body = body
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("X-Correlation-Id", "correlation-123")

	handler := NewBuiltinMiddleware().Logging()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != body {
			t.Fatal("logging middleware replaced the request body")
		}
		SetRequestLogMetadata(r, "upload-route", "upload-backend")
		gotBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(gotBody) != "streamed request body" {
			t.Fatalf("request body = %q", gotBody)
		}
		w.WriteHeader(http.StatusCreated)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	for _, field := range []string{
		"method=\"POST\"", "uri=\"/upload\"", "status=201",
		"route=\"upload-route\"", "backend=\"upload-backend\"",
		"correlation_id=\"correlation-123\"",
	} {
		if !strings.Contains(output.String(), field) {
			t.Errorf("log %q does not contain %q", output.String(), field)
		}
	}
}

func TestLoggingPreservesFlushedResponseBodyAndStatus(t *testing.T) {
	var output bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	}()

	handler := NewBuiltinMiddleware().Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first "))
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("second"))
		w.WriteHeader(http.StatusCreated)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d after flush", response.Code, http.StatusOK)
	}
	if response.Body.String() != "first second" {
		t.Fatalf("response body = %q, want streamed chunks unchanged", response.Body.String())
	}
	if !strings.Contains(output.String(), "status=200") {
		t.Fatalf("log %q does not report flushed status 200", output.String())
	}
}

func TestLoggingCapturesFinalStatusAfterInformationalResponse(t *testing.T) {
	var output bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	}()

	handler := NewBuiltinMiddleware().Logging()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.WriteHeader(http.StatusCreated)
	}))
	response := &informationalResponseWriter{}
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.status != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.status, http.StatusCreated)
	}
	if len(response.codes) != 2 || response.codes[0] != http.StatusEarlyHints || response.codes[1] != http.StatusCreated {
		t.Fatalf("forwarded statuses = %v, want [103 201]", response.codes)
	}
	if !strings.Contains(output.String(), "status=201") {
		t.Fatalf("log %q does not report final status 201", output.String())
	}
}
