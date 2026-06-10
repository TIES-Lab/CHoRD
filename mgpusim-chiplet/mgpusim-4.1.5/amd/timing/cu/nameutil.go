package cu

import (
	"strconv"
	"strings"
)

// parseBracketedID parses an integer that appears after a token like "SA[".
// It returns (value, true) on success.
func parseBracketedID(name, token string) (int, bool) {
	idx := strings.Index(name, token)
	if idx < 0 {
		return 0, false
	}
	idx += len(token)
	end := idx
	for end < len(name) {
		c := name[end]
		if c < '0' || c > '9' {
			break
		}
		end++
	}
	if end == idx {
		return 0, false
	}
	if end >= len(name) || name[end] != ']' {
		return 0, false
	}
	v, err := strconv.Atoi(name[idx:end])
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseGlobalSAIDFromCUName(name string) (int, bool) {
	return parseBracketedID(name, "SA[")
}
