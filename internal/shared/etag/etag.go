// Package etag formats and parses the strong ETag validators of the JSON API.
package etag

import (
	"fmt"
	"strconv"
	"strings"
)

// Format returns the strong validator for one resource kind at a version.
func Format(kind string, version uint64) string {
	return fmt.Sprintf(`"%s:%d"`, kind, version)
}

// Parse extracts the version from a validator of the given kind. It accepts
// only the exact validator Format emits, so a malformed or re-spelled value
// never matches, and rejects version zero.
func Parse(kind, value string) (uint64, bool) {
	return parse(kind, value, false)
}

// ParseAllowZero extracts the version from a validator of the given kind,
// allowing version zero for resources that start in an uninitialized state.
func ParseAllowZero(kind, value string) (uint64, bool) {
	return parse(kind, value, true)
}

func parse(kind, value string, allowZero bool) (uint64, bool) {
	prefix := `"` + kind + `:`
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, `"`) {
		return 0, false
	}
	version, err := strconv.ParseUint(value[len(prefix):len(value)-1], 10, 64)
	if err != nil || (!allowZero && version == 0) || value != Format(kind, version) {
		return 0, false
	}
	return version, true
}
