package catalog

import (
	"errors"
	"testing"
)

func TestParseMappingETag(t *testing.T) {
	t.Parallel()

	version, star, ok := ParseMappingETag("*")
	if !ok || !star || version != 0 {
		t.Fatalf("star %d %v %v", version, star, ok)
	}
	version, star, ok = ParseMappingETag(`"listing-mapping:3"`)
	if !ok || star || version != 3 {
		t.Fatalf("version %d %v %v", version, star, ok)
	}
	if _, _, ok := ParseMappingETag(`"catalog-pin:1"`); ok {
		t.Fatal("accepted foreign etag")
	}
}

func TestParsePinETag(t *testing.T) {
	t.Parallel()

	version, star, ok := ParsePinETag("*")
	if !ok || !star || version != 0 {
		t.Fatalf("star %d %v %v", version, star, ok)
	}
	version, star, ok = ParsePinETag(`"catalog-pin:8"`)
	if !ok || star || version != 8 {
		t.Fatalf("version %d %v %v", version, star, ok)
	}
	if _, _, ok := ParsePinETag(`"listing-mapping:1"`); ok {
		t.Fatal("accepted foreign etag")
	}
}

func TestNormalizeSubjectKind(t *testing.T) {
	t.Parallel()

	kind, err := normalizeSubjectKind(subjectListing)
	if err != nil || kind != subjectListing {
		t.Fatalf("listing %s %v", kind, err)
	}
	if _, err := normalizeSubjectKind("account"); !errors.Is(err, ErrInvalidSubject) {
		t.Fatalf("account %v", err)
	}
}
