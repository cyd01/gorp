package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"math/rand"
	"net/http"
	"strconv"
)

func NewCookieSelector(backends []*backend.Backend, name string) *AffinitySelector {
	return NewAffinitySelector(backends, name, func(req *http.Request) string {
		value := ""
		cookie, err := req.Cookie(name)
		if (err != nil) || (cookie.Value == "") {
			value = strconv.Itoa(rand.Intn(len(backends)))
		}
		return value
	})
}
