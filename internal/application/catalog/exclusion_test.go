package catalog

import "testing"

func TestExclusionETagRoundTrip(t *testing.T) {
	t.Parallel()

	for _, version := range []uint64{0, 1, 2, 42} {
		l := Listing{ExclusionVersion: version}
		val := l.ExclusionETag()
		got, ok := ParseExclusionETag(val)
		if !ok || got != version {
			t.Fatalf("ParseExclusionETag(%q) = %d, %v; want %d, true", val, got, ok, version)
		}
	}
}

func TestParseExclusionETagRejectsMalformed(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"",
		`"listing-exclusion:"`,
		`"listing-exclusion:01"`,
		`"listing-exclusion:0"x`,
		`W/"listing-exclusion:0"`,
		`"listing:0"`,
	} {
		if version, ok := ParseExclusionETag(value); ok {
			t.Fatalf("ParseExclusionETag(%q) = %d, true; want rejected", value, version)
		}
	}
}
