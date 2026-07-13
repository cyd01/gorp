package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
)

func available(backends []*backend.Backend) []*backend.Backend {
	out := make([]*backend.Backend, 0, len(backends))
	for _, b := range backends {
		if b.Available() {
			out = append(out, b)
		}
	}
	return out
}
