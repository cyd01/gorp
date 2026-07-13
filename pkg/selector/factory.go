package selector

import (
	"github.com/cyd01/gorp/pkg/backend"
	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/helper"
	"hash/fnv"
	"strings"
)

func New(selector config.SelectorConfig, backends []*backend.Backend) Selector {
	switch strings.ToLower(selector.Type) {
	case "cookie":
		cookie := helper.ParseStringOrDefault(selector.Config, "name", "SESSIONID")
		return NewCookieSelector(backends, cookie)
	case "first-alive", "first", "alive":
		return NewFirstAlive(backends)
	case "header":
		header := helper.ParseStringOrDefault(selector.Config, "name", "Affinity")
		return NewHeaderSelector(backends, header)
	case "ip", "source-ip", "sourceip", "ip-hash", "iphash":
		return NewIPSelector(backends)
	case "leastconnection", "leastconnections", "least-connection", "least-connections":
		return NewLeastConnections(backends)
	case "power-of-two", "poweroftwo", "p2c":
		return NewPowerOfTwoChoices(backends)
	case "query", "query-param", "queryparam":
		param := helper.ParseStringOrDefault(selector.Config, "name", "SESSIONID")
		return NewCookieSelector(backends, param)
	case "rand", "random":
		return NewRandomSelector(backends)
	default:
		return NewRoundRobin(backends)
	}
}

func hashIndex(key string, n int) int {
	if n == 1 {
		return 0
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % n
}
