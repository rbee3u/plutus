package capital

import "testing"

func TestParseBaselineETag(t *testing.T) {
	version, star, ok := ParseBaselineETag("*")
	if !ok || !star || version != 0 {
		t.Fatalf("star %d %v %v", version, star, ok)
	}
	version, star, ok = ParseBaselineETag(`"capital-baseline:12"`)
	if !ok || star || version != 12 {
		t.Fatalf("version %d %v %v", version, star, ok)
	}
	if _, _, ok := ParseBaselineETag(`"capital-flow:1"`); ok {
		t.Fatal("accepted foreign etag")
	}
}

func TestPositiveDecimal(t *testing.T) {
	if _, err := positiveDecimal("0"); err == nil {
		t.Fatal("zero quantity")
	}
	if _, err := positiveDecimal(""); err == nil {
		t.Fatal("empty quantity")
	}
	got, err := positiveDecimal("25.50")
	if err != nil || got != "25.50" {
		t.Fatalf("positive %s %v", got, err)
	}
	if compactDecimal("25.50") != "25.5" {
		t.Fatalf("compact %s", compactDecimal("25.50"))
	}
}
