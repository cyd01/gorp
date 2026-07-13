package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"net"
	"net/http"
)

func NewIPSelector(backends []*backend.Backend) *AffinitySelector {
	return &AffinitySelector{
		backends: backends,
		key: func(req *http.Request) string {
			host, _, err := net.SplitHostPort(req.RemoteAddr)
			if err != nil {
				host = req.RemoteAddr
			}
			return host
		}}
}
