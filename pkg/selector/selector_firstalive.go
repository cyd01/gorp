package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"net/http"
)

type FirstAlive struct {
	backends []*backend.Backend
}

func NewFirstAlive(backends []*backend.Backend) *FirstAlive {
	return &FirstAlive{backends: backends}
}

func (s *FirstAlive) Select(_ *http.Request) (*backend.Backend, error) {
	if len(s.backends) == 0 {
		return nil, ErrNoBackend
	}
	available := available(s.backends)
	if len(available) == 0 {
		return nil, ErrNoValidBackend
	}
	return available[0], nil
}
