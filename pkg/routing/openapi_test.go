package routing

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAPIMiddlewareValidatesRequests(t *testing.T) {
	contract := []byte(`openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /pets:
    get:
      parameters:
        - name: limit
          in: query
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: OK
`)
	filename := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(filename, contract, 0600); err != nil {
		t.Fatal(err)
	}
	middleware, err := OpenAPIMiddleware(filename)
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := middleware(next)

	valid := httptest.NewRequest(http.MethodGet, "/pets?limit=10", nil)
	validResponse := httptest.NewRecorder()
	handler.ServeHTTP(validResponse, valid)
	if validResponse.Code != http.StatusNoContent {
		t.Fatalf("valid request status = %d, want %d", validResponse.Code, http.StatusNoContent)
	}

	invalid := httptest.NewRequest(http.MethodGet, "/pets", nil)
	invalidResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid request status = %d, want %d", invalidResponse.Code, http.StatusBadRequest)
	}
}

func TestOpenAPIMiddlewareRejectsInvalidContract(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "invalid.yaml")
	if err := os.WriteFile(filename, []byte("openapi: 3.1.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAPIMiddleware(filename); err == nil {
		t.Fatal("OpenAPIMiddleware() accepted an invalid contract")
	}
}
