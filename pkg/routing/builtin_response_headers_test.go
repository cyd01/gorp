package routing

import (
	"net/http"
	"testing"
)

func TestAddResponseLocationPrefix(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     string
	}{
		{name: "missing location", want: ""},
		{name: "path", location: "/login?next=/home#form", want: "/prefix/login?next=/home#form"},
		{name: "absolute http uri", location: "http://backend.example/login", want: "http://backend.example/prefix/login"},
		{name: "absolute https uri", location: "https://backend.example/login", want: "https://backend.example/prefix/login"},
		{name: "prefix slash", location: "/login", want: "/prefix/login"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := &http.Response{Header: make(http.Header)}
			if test.location != "" {
				response.Header.Set("Location", test.location)
			}

			if err := (&BuiltinMiddleware{}).AddResponseLocationPrefix("/prefix/")(response); err != nil {
				t.Fatalf("AddResponseLocationPrefix() error = %v", err)
			}
			if got := response.Header.Get("Location"); got != test.want {
				t.Fatalf("Location = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSetResponseLocationHost(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     string
	}{
		{name: "missing location", want: ""},
		{name: "relative path", location: "/login", want: "/login"},
		{name: "absolute http uri", location: "http://backend.example/login?next=/home#form", want: "http://public.example/login?next=/home#form"},
		{name: "absolute https uri", location: "https://backend.example:8443/login", want: "https://public.example/login"},
		{name: "other scheme", location: "ftp://backend.example/file", want: "ftp://public.example/file"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := &http.Response{Header: make(http.Header)}
			if test.location != "" {
				response.Header.Set("Location", test.location)
			}

			if err := (&BuiltinMiddleware{}).SetResponseLocationHost("public.example")(response); err != nil {
				t.Fatalf("SetResponseLocationHost() error = %v", err)
			}
			if got := response.Header.Get("Location"); got != test.want {
				t.Fatalf("Location = %q, want %q", got, test.want)
			}
		})
	}
}
