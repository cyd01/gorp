package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"math/rand"
	"net/http"
	"strconv"
)

func NewQueryParamSelector(backends []*backend.Backend, name string) *AffinitySelector {
	return NewAffinitySelector(backends, name, func(req *http.Request) string {
		value := req.URL.Query().Get(name)
		if value == "" {
			value = strconv.Itoa(rand.Intn(len(backends)))
		}
		return value
	})
}
