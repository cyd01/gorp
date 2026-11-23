package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"net/http"
)

type AffinitySelector struct {
	backends     []*backend.Backend
	name         string
	key          func(*http.Request) string
	backendIndex func(*http.Request) (int, bool)
}

func (s *AffinitySelector) Select(req *http.Request) (*backend.Backend, error) {
	if len(s.backends) == 0 {
		return nil, ErrNoBackend
	}
	var index int
	direct := false
	if s.backendIndex != nil {
		index, direct = s.backendIndex(req)
	}
	if !direct {
		index = hashIndex(s.key(req), len(s.backends))
	}
	for offset := 0; offset < len(s.backends); offset++ {
		candidate := s.backends[(index+offset)%len(s.backends)]
		if candidate.Available() {
			return candidate, nil
		}
	}
	return nil, ErrNoValidBackend
}

func NewAffinitySelector(backends []*backend.Backend, name string, key func(*http.Request) string) *AffinitySelector {
	return &AffinitySelector{
		backends: backends,
		name:     name,
		key:      key,
	}
}
