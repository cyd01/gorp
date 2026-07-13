package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"net/http"
	"sync"
)

type RoundRobinSelector struct {
	backends []*backend.Backend
	index    int
	mutex    sync.Mutex
}

func (r *RoundRobinSelector) Select(_ *http.Request) (*backend.Backend, error) {
	if len(r.backends) == 0 {
		return nil, ErrNoBackend
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i := 0; i < len(r.backends); i++ {
		b := r.backends[r.index]
		r.index++
		if r.index >= len(r.backends) {
			r.index = 0
		}
		if b.Available() {
			return b, nil
		}
	}
	return nil, ErrNoValidBackend
}

func NewRoundRobin(backends []*backend.Backend) *RoundRobinSelector {
	return &RoundRobinSelector{backends: backends}
}
