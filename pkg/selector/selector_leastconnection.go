package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"math"
	"math/rand"
	"net/http"
)

type LeastConnections struct {
	backends []*backend.Backend
}

func NewLeastConnections(backends []*backend.Backend) *LeastConnections {
	return &LeastConnections{backends: backends}
}

func (s *LeastConnections) Select(_ *http.Request) (*backend.Backend, error) {
	if len(s.backends) == 0 {
		return nil, ErrNoBackend
	}
	bestLoad := int64(math.MaxInt64)
	var candidates []*backend.Backend
	available := available(s.backends)
	if len(available) == 0 {
		return nil, ErrNoValidBackend
	}
	for _, b := range available {
		load := b.ActiveConnections.Load()
		switch {
		case load < bestLoad:
			bestLoad = load
			candidates = []*backend.Backend{b}
		case load == bestLoad:
			candidates = append(candidates, b)
		}
	}
	if len(candidates) == 0 {
		return nil, ErrNoValidBackend
	}
	return candidates[rand.Intn(len(candidates))], nil
}
