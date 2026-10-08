package decimal

import (
	"strings"
	"testing"
)

func TestFormatCompactsTrailingZeros(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ input, want string }{
		{"1234.5000", "1234.5"},
		{"0.000", "0"},
		{"-0.000", "0"},
		{"-0.0100", "-0.01"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			rat, ok := Parse(tc.input)
			if !ok {
				t.Fatalf("Parse(%q) failed", tc.input)
			}
			if got := Format(rat); got != tc.want {
				t.Fatalf("Format(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
	if got := Format(Mul(ParseOrZero("0.5"), ParseOrZero("65000"))); got != "32500" {
		t.Fatalf("mul got %q", got)
	}
}

func TestValidDecimalStorageBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input string
		want        bool
	}{
		{"zero", "0", true},
		{"negative", "-1.25", true},
		{"surrounding whitespace", " \t-1.25\n", true},
		{"full precision", strings.Repeat("9", 35) + "." + strings.Repeat("9", 30), true},
		{"smallest fraction", "0." + strings.Repeat("0", 29) + "1", true},
		{"integer overflow", strings.Repeat("9", 36), false},
		{"fraction overflow", "0." + strings.Repeat("0", 30) + "1", false},
		{"blank", "", false},
		{"exponent", "1e5", false},
		{"text", "abc", false},
		{"missing fraction", "1.", false},
		{"missing integer", ".5", false},
		{"double sign", "--1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Valid(tc.input); got != tc.want {
				t.Fatalf("Valid(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatPtrDistinguishesUnknownFromZero(t *testing.T) {
	t.Parallel()
	if got := FormatPtr(nil); got != nil {
		t.Fatalf("unknown formatted as %q", *got)
	}
	if got := FormatPtr(Zero()); got == nil || *got != "0" {
		t.Fatalf("zero formatted as %v", got)
	}
}

func FuzzDecimalRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"0", "-0.000", "1234.5000", " \t-1.25\n",
		"65000.000000000000000000000000000001",
		strings.Repeat("9", 35) + "." + strings.Repeat("9", 30),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		// Parse also accepts fractions and exponents; only storage decimals are
		// guaranteed to survive formatting without rounding.
		if !Valid(input) {
			t.Skip()
		}
		value, ok := Parse(input)
		if !ok {
			t.Fatalf("valid decimal %q did not parse", input)
		}
		formatted := Format(value)
		roundTrip, ok := Parse(formatted)
		if !ok || value.Cmp(roundTrip) != 0 {
			t.Fatalf("formatting %q as %q changed its value", input, formatted)
		}
		if !Valid(formatted) || Format(roundTrip) != formatted || Compact(formatted) != formatted {
			t.Fatalf("Format(%q) = %q is not a canonical storage decimal", input, formatted)
		}
	})
}
