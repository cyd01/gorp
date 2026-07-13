package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"math/rand"
	"net/http"
)

type RandomSelector struct {
	backends []*backend.Backend
}

func (r *RandomSelector) Select(_ *http.Request) (*backend.Backend, error) {
	if len(r.backends) == 0 {
		return nil, ErrNoBackend
	}
	candidates := available(r.backends)
	if len(candidates) == 0 {
		return nil, ErrNoValidBackend
	}
	return candidates[rand.Intn(len(candidates))], nil
}

func NewRandomSelector(backends []*backend.Backend) *RandomSelector {
	return &RandomSelector{backends: backends}
}
