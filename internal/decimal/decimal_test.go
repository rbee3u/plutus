package decimal

import "testing"

func TestFormatCompactsTrailingZeros(t *testing.T) {
	t.Parallel()
	rat, ok := Parse("1234.5000")
	if !ok {
		t.Fatal("parse")
	}
	if got := Format(rat); got != "1234.5" {
		t.Fatalf("got %q", got)
	}
	if got := Format(MustParse("0.000")); got != "0" {
		t.Fatalf("zero got %q", got)
	}
	if got := Format(Mul(MustParse("0.5"), MustParse("65000"))); got != "32500" {
		t.Fatalf("mul got %q", got)
	}
}

func TestValidRejectsExponentsAndBlanks(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"", "1e5", "abc", "1.", ".5", "--1"} {
		if Valid(bad) {
			t.Fatalf("%q should be invalid", bad)
		}
	}
	for _, good := range []string{"0", "-1.25", "65000.000000000000000000000000000001"} {
		if !Valid(good) {
			t.Fatalf("%q should be valid", good)
		}
	}
}
