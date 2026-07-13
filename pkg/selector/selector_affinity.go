package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"net/http"
)

type AffinitySelector struct {
	backends []*backend.Backend
	name     string
	key      func(*http.Request) string
}

func (s *AffinitySelector) Select(req *http.Request) (*backend.Backend, error) {
	if len(s.backends) == 0 {
		return nil, ErrNoBackend
	}
	b := s.backends[hashIndex(s.key(req), len(s.backends))]
	if b.Available() {
		return b, nil
	} else {
		return nil, ErrNoValidBackend
	}
}

func NewAffinitySelector(backends []*backend.Backend, name string, key func(*http.Request) string) *AffinitySelector {
	return &AffinitySelector{
		backends: backends,
		name:     name,
		key:      key,
	}
}
