package routing

import (
	"path/filepath"
	"strings"
)

func MatchWildcard(pattern, value string) bool {
	ok, err := filepath.Match(pattern, value)
	if err != nil {
		return false
	}
	return ok
}

// MatchPath matches a path pattern against a path.
// It supports :name path parameters, * and ? wildcards for single segments,
// and ** for multi-segment wildcards.
func MatchPath(pattern, path string) (map[string]string, bool) {
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(path, "/")

	var match func(pIdx, sIdx int, currentValues map[string]string) (map[string]string, bool)
	match = func(pIdx, sIdx int, currentValues map[string]string) (map[string]string, bool) {
		if pIdx == len(patternSegments) {
			if sIdx == len(pathSegments) {
				return currentValues, true
			}
			return nil, false
		}

		pSeg := patternSegments[pIdx]

		// Handle **
		if pSeg == "**" {
			// Try matching zero segments
			if res, ok := match(pIdx+1, sIdx, currentValues); ok {
				return res, true
			}
			// Try matching one or more segments
			for i := sIdx; i < len(pathSegments); i++ {
				// If it's the last segment, it matches everything left
				if pIdx == len(patternSegments)-1 {
					return currentValues, true
				}
				// Try to match the rest of the pattern starting from the next segment
				if res, ok := match(pIdx+1, i+1, currentValues); ok {
					return res, true
				}
			}
			return nil, false
		}

		// If we reached end of path but not pattern
		if sIdx == len(pathSegments) {
			return nil, false
		}

		sSeg := pathSegments[sIdx]

		// Handle :name
		if strings.HasPrefix(pSeg, ":") {
			if sSeg == "" {
				return nil, false
			}
			newValues := make(map[string]string, len(currentValues)+1)
			for k, v := range currentValues {
				newValues[k] = v
			}
			newValues[pSeg[1:]] = sSeg
			return match(pIdx+1, sIdx+1, newValues)
		}

		// Handle wildcard * or ? or MatchWildcard
		if MatchWildcard(pSeg, sSeg) {
			return match(pIdx+1, sIdx+1, currentValues)
		}

		return nil, false
	}

	return match(0, 0, make(map[string]string))
}
