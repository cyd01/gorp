package routing

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestChainMiddlewareOrder(t *testing.T) {
	var calls []string
	middleware := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, name+" before")
				next.ServeHTTP(w, r)
				calls = append(calls, name+" after")
			})
		}
	}

	handler := ChainMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls = append(calls, "handler")
	}), middleware("first"), middleware("second"))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"first before", "second before", "handler", "second after", "first after"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("middleware order = %v, want %v", calls, want)
	}
}

func TestMiddlewareRegistry(t *testing.T) {
	registry := NewMiddlewareRegistry()
	first := Middleware(func(next http.Handler) http.Handler { return next })
	second := Middleware(func(next http.Handler) http.Handler { return next })
	registry.Register("first", first)
	registry.Register("second", second)

	got, ok := registry.Get("first")
	if !ok || reflect.ValueOf(got).Pointer() != reflect.ValueOf(first).Pointer() {
		t.Fatal("Get(first) did not return the registered middleware")
	}
	if _, ok := registry.Get("missing"); ok {
		t.Fatal("Get(missing) unexpectedly succeeded")
	}

	all := registry.GetAll([]string{"second", "missing", "first"})
	if len(all) != 2 ||
		reflect.ValueOf(all[0]).Pointer() != reflect.ValueOf(second).Pointer() ||
		reflect.ValueOf(all[1]).Pointer() != reflect.ValueOf(first).Pointer() {
		t.Fatalf("GetAll() returned unexpected middleware: %v", all)
	}
}

func TestCorrelationIDMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		requestID  string
		generateID bool
	}{
		{name: "generates missing id", generateID: true},
		{name: "preserves existing id", requestID: "existing-correlation-id"},
		{name: "replaces empty id", requestID: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.requestID != "" || test.name == "replaces empty id" {
				request.Header.Set("X-Correlation-Id", test.requestID)
			}
			response := httptest.NewRecorder()
			called := false
			handler := NewBuiltinMiddleware().CorrelationID("uuidv7")(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				called = true
				if got := r.Header.Get("X-Correlation-Id"); got == "" {
					t.Error("request correlation ID is empty")
				} else if test.generateID {
					if _, err := uuid.Parse(got); err != nil {
						t.Errorf("generated correlation ID %q is invalid: %v", got, err)
					}
				} else if test.requestID != "" && got != test.requestID {
					t.Errorf("request correlation ID = %q, want %q", got, test.requestID)
				}
			}))

			handler.ServeHTTP(response, request)
			if !called {
				t.Fatal("next handler was not called")
			}
			if got := response.Header().Get("X-Correlation-Id"); got == "" {
				t.Fatal("response correlation ID is empty")
			} else if test.requestID != "" && got != test.requestID {
				t.Fatalf("response correlation ID = %q, want %q", got, test.requestID)
			}
		})
	}
}

func TestCORSPreflightAndRegularRequest(t *testing.T) {
	for _, method := range []string{http.MethodOptions, http.MethodGet} {
		t.Run(method, func(t *testing.T) {
			called := false
			handler := NewBuiltinMiddleware().CORS()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(method, "/", nil))

			if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
				t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
			}
			if got := response.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, DELETE, OPTIONS" {
				t.Errorf("Access-Control-Allow-Methods = %q", got)
			}
			if got := response.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization, X-Requested-With" {
				t.Errorf("Access-Control-Allow-Headers = %q", got)
			}
			if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
				t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
			}

			if method == http.MethodOptions {
				if response.Code != http.StatusOK || called {
					t.Fatalf("preflight response status/called = %d/%t, want %d/false", response.Code, called, http.StatusOK)
				}
			} else if !called {
				t.Fatal("next handler was not called for a regular request")
			}
		})
	}
}

func TestAddPrefix(t *testing.T) {
	for _, prefix := range []string{"api", "/api"} {
		t.Run(prefix, func(t *testing.T) {
			var paths []string
			handler := NewBuiltinMiddleware().AddPrefix(prefix)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
			}))

			for range 2 {
				handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items", nil))
			}
			want := []string{"/api/items", "/api/items"}
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("request paths = %v, want %v", paths, want)
			}
		})
	}
}

func TestIPWhitelist(t *testing.T) {
	called := 0
	handler := NewBuiltinMiddleware().IPWhitelist([]string{"192.0.2.0/24", "2001:db8::/32", "invalid"})(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called++ }),
	)
	tests := []struct {
		name       string
		remoteAddr string
		wantStatus int
	}{
		{name: "allowed ipv4", remoteAddr: "192.0.2.10:1234", wantStatus: http.StatusOK},
		{name: "allowed ipv6", remoteAddr: "[2001:db8::1]:1234", wantStatus: http.StatusOK},
		{name: "outside whitelist", remoteAddr: "198.51.100.10:1234", wantStatus: http.StatusForbidden},
		{name: "invalid remote address", remoteAddr: "not-an-ip", wantStatus: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = test.remoteAddr
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
	if called != 2 {
		t.Fatalf("next handler called %d times, want 2", called)
	}
}

func TestRequestHeaderMiddlewares(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Add("X-Append", "existing")
	request.Header.Set("X-Set", "old")
	request.Header.Set("X-Replace", "old")
	request.Header.Set("X-Remove", "remove")
	response := httptest.NewRecorder()
	handler := ChainMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := r.Header.Values("X-Append"); !reflect.DeepEqual(got, []string{"existing", "added"}) {
			t.Errorf("X-Append = %v, want [existing added]", got)
		}
		for header, want := range map[string]string{"X-Set": "new", "X-Replace": "replacement"} {
			if got := r.Header.Get(header); got != want {
				t.Errorf("%s = %q, want %q", header, got, want)
			}
		}
		if got := r.Header.Get("X-Remove"); got != "" {
			t.Errorf("X-Remove = %q, want empty", got)
		}
	}),
		NewBuiltinMiddleware().AddRequestHeaders(map[string]string{"X-Append": "added"}),
		NewBuiltinMiddleware().SetRequestHeader(map[string]string{"X-Set": "new"}),
		NewBuiltinMiddleware().ModifyRequestHeaders(map[string]string{"X-Replace": "replacement"}),
		NewBuiltinMiddleware().RemoveRequestHeaders([]string{"X-Remove"}),
	)
	handler.ServeHTTP(response, request)
}
