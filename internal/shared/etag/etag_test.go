package etag

import "testing"

func TestFormatParseRoundTrip(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"identity", "capital-flow", "listing-exclusion"} {
		for _, version := range []uint64{1, 2, 42, 1 << 40} {
			value := Format(kind, version)
			got, ok := Parse(kind, value)
			if !ok || got != version {
				t.Fatalf("Parse(%q, %q) = %d, %v; want %d, true", kind, value, got, ok, version)
			}
		}
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"",
		`"identity:1`,
		`identity:1"`,
		`"identity:"`,
		`"identity:0"`,
		`"identity:01"`,
		`"identity:1x"`,
		`"identity:-1"`,
		`"identity:1"x`,
		`W/"identity:1"`,
		`"portfolio:1"`,
		`"identity:18446744073709551616"`,
	} {
		if version, ok := Parse("identity", value); ok {
			t.Fatalf("Parse(%q) = %d, true; want rejected", value, version)
		}
	}
}
