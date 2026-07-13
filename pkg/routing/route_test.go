package routing

import (
	"net/http"
	"testing"
)

func TestRouteMatchEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []string
		endpoint  string
		want      bool
	}{
		{name: "no endpoint restriction", endpoint: "https", want: true},
		{name: "matching endpoint", endpoints: []string{"http", "https"}, endpoint: "https", want: true},
		{name: "non matching endpoint", endpoints: []string{"http"}, endpoint: "https", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			route := &Route{Endpoints: test.endpoints}
			if got := route.MatchEndpoint(test.endpoint); got != test.want {
				t.Fatalf("MatchEndpoint() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRouteMatchPath(t *testing.T) {
	tests := []struct {
		name   string
		paths  []string
		prefix string
		hosts  []string
		path   string
		host   string
		want   bool
	}{
		{name: "matching path", paths: []string{"/*.json"}, path: "/data.json", want: true},
		{name: "matching filename pattern", paths: []string{"*.mp4"}, path: "/videos/movie.mp4", want: true},
		{name: "path pattern does not match another extension", paths: []string{"*.mp4"}, path: "/videos/movie.webm", want: false},
		{name: "non matching path", paths: []string{"/*.json"}, path: "/data.xml", want: false},
		{name: "path is an alternative to prefix", paths: []string{"/*.json"}, prefix: "/api", path: "/data.json", want: true},
		{name: "existing host matching is preserved", paths: []string{"/*.json"}, hosts: []string{"api.example.com"}, path: "/data.xml", host: "api.example.com", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://"+test.host+test.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			route := &Route{Paths: test.paths, Prefix: test.prefix, Hosts: test.hosts}
			if got := route.Match(req); got != test.want {
				t.Fatalf("Match() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRouterPrioritizesMatchingPath(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com/videos/movie.mp4", nil)
	if err != nil {
		t.Fatal(err)
	}
	router := New([]*Route{
		{Name: "simple", Prefix: "/"},
		{Name: "video", Paths: []string{"*.mp4"}},
	}, "")

	if got := router.routeFor(req, ""); got == nil || got.Name != "video" {
		if got == nil {
			t.Fatal("routeFor() returned nil, want video route")
		}
		t.Fatalf("routeFor() returned %q, want video route", got.Name)
	}
}
