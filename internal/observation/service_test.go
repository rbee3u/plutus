package observation

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestCreateRequiresOneSubtypeAndEquityCoverage(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()
	portfolioID := uuid.Must(uuid.NewV7())

	_, err := svc.Create(ctx, portfolioID, &CreateInput{Kind: "balance"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing subtype: %v", err)
	}

	_, err = svc.Create(ctx, portfolioID, &CreateInput{
		Kind: "reported_equity",
		ReportedEquity: &ReportedEquity{
			DenominationListingID: uuid.Must(uuid.NewV7()),
			Amount:                "100",
		},
	})
	if !errors.Is(err, ErrCoverageRequired) {
		t.Fatalf("coverage: %v", err)
	}
}
