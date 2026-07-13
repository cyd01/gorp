package httpmulti

import (
	"regexp"
)

func isValidAddr(addr string) bool {
	regex := regexp.MustCompile(`^([a-zA-Z0-9.-]+)?:([0-9]+)$`)
	return regex.MatchString(addr)
}
