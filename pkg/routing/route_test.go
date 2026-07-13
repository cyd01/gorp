package routing

import "testing"

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
