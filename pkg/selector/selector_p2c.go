package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"math/rand"
	"net/http"
)

type PowerOfTwoChoices struct {
	backends []*backend.Backend
}

func NewPowerOfTwoChoices(backends []*backend.Backend) *PowerOfTwoChoices {
	return &PowerOfTwoChoices{backends: backends}
}

func (s *PowerOfTwoChoices) Select(_ *http.Request) (*backend.Backend, error) {
	if len(s.backends) == 0 {
		return nil, ErrNoBackend
	}
	available := available(s.backends)

	switch len(available) {
	case 0:
		return nil, ErrNoValidBackend
	case 1:
		return available[0], nil
	}

	i := rand.Intn(len(available))
	j := rand.Intn(len(available) - 1)
	if j >= i {
		j++
	}

	a := available[i]
	b := available[j]

	if a.ActiveConnections.Load() <= b.ActiveConnections.Load() {
		return a, nil
	}

	return b, nil
}
