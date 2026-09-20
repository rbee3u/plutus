package observation

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestDecodeObservationCursorRejectsForeignPortfolio(t *testing.T) {
	t.Parallel()

	portfolioID := uuid.MustParse("01a0bdfc-1fa4-7a54-9fa1-24f4721a71af")
	other := uuid.MustParse("01a0bdfc-1fa4-7af4-bedf-65d968b9122b")
	encoded, err := encodeCursor(listCursor{PortfolioID: portfolioID, ObservedAt: 1, ID: portfolioID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCursor(encoded, other); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("foreign cursor %v", err)
	}
}
