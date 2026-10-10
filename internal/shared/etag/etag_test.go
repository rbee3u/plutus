package etag

import "testing"

func TestFormatParseRoundTrip(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"identity", "capital-flow", "asset-exclusion"} {
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

func TestParseAllowZero(t *testing.T) {
	t.Parallel()

	if version, ok := ParseAllowZero("asset-exclusion", `"asset-exclusion:0"`); !ok || version != 0 {
		t.Fatalf("ParseAllowZero() = %d, %v; want 0, true", version, ok)
	}
	if version, ok := Parse("asset-exclusion", `"asset-exclusion:0"`); ok {
		t.Fatalf("Parse() = %d, true; want rejected", version)
	}

	for _, value := range []string{
		"",
		`"asset-exclusion:"`,
		`"asset-exclusion:01"`,
		`"asset-exclusion:-0"`,
		`"asset-exclusion:0"x`,
		`W/"asset-exclusion:0"`,
	} {
		if version, ok := ParseAllowZero("asset-exclusion", value); ok {
			t.Fatalf("ParseAllowZero(%q) = %d, true; want rejected", value, version)
		}
	}
}
