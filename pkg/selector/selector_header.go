package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

const backendAffinityPrefix = "__gorp_backend_index:"

// BackendAffinityKey returns the opaque header value used to keep a request on
// the backend that generated its response.
func BackendAffinityKey(index int) string {
	return backendAffinityPrefix + strconv.Itoa(index)
}

func NewHeaderSelector(backends []*backend.Backend, name string) *AffinitySelector {
	selector := NewAffinitySelector(backends, name, func(req *http.Request) string {
		value := req.Header.Get(name)
		if value == "" {
			value = strconv.Itoa(rand.Intn(len(backends)))
		}
		return value
	})
	selector.backendIndex = func(req *http.Request) (int, bool) {
		value := req.Header.Get(name)
		if !strings.HasPrefix(value, backendAffinityPrefix) {
			return 0, false
		}
		index, err := strconv.Atoi(strings.TrimPrefix(value, backendAffinityPrefix))
		return index, err == nil && index >= 0 && index < len(backends)
	}
	return selector
}
