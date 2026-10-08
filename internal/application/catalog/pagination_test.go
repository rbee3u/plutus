package catalog

import (
	"errors"
	"math"
	"testing"
)

func TestPaginationOffsetBounds(t *testing.T) {
	t.Parallel()
	limit, offset, err := normalizePage(1, math.MaxInt32)
	if err != nil || limit != 1 || offset != math.MaxInt32 {
		t.Fatalf("SQL int32 boundary rejected: limit=%d offset=%d err=%v", limit, offset, err)
	}
	if offset == math.MaxInt {
		t.Skip("int cannot represent an offset above MaxInt32")
	}
	offset++
	if _, _, err := normalizePage(1, offset); !errors.Is(err, ErrInvalid) {
		t.Fatalf("overflowing SQL offset accepted: %v", err)
	}
}
