package routing

import (
	"path/filepath"
)

func MatchWildcard(pattern, value string) bool {
	ok, err := filepath.Match(pattern, value)
	if err != nil {
		return false
	}
	return ok
}
