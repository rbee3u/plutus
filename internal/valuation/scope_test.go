package valuation

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/decimal"
)

func TestScopeChangeDoesNotInventCapitalFlows(t *testing.T) {
	t.Parallel()
	identityID, earnID, dualID := uuid.New(), uuid.New(), uuid.New()
	old := map[uuid.UUID]previousAccount{earnID: {name: "Earn", accountType: "earn", scopeVersion: 1, identityID: identityID, platform: "okx", method: MethodCalculated, value: decimal.MustParse("100")}}
	makeAccount := func(id uuid.UUID, kind, amount string) AccountValuation {
		a := AccountValuation{AccountID: id, IdentityID: identityID, IdentityName: "OKX", Platform: "okx", AccountType: kind, ScopeVersion: 2, Method: MethodCalculated, Freshness: FreshnessFresh}
		if amount != "" {
			a.valueRat = decimal.MustParse(amount)
			a.ValueUSD = &amount
		} else {
			a.Method = MethodMissing
		}
		return a
	}
	for _, tc := range []struct {
		name     string
		accounts []AccountValuation
		total    string
		events   int
		changed  bool
	}{
		{"split", []AccountValuation{makeAccount(earnID, "earn", "60"), makeAccount(dualID, "dual_investment", "40")}, "100", 0, true},
		{"first Earn failure", []AccountValuation{makeAccount(earnID, "earn", ""), makeAccount(dualID, "dual_investment", "40")}, "40", 0, true},
		{"only Dual remains", []AccountValuation{makeAccount(dualID, "dual_investment", "40")}, "40", 0, true},
		{"whole slice deselected", nil, "0", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			point := buildPoint(uuid.New(), uuid.New(), time.Now(), tc.accounts, old)
			changed := false
			for _, issue := range point.Issues {
				changed = changed || issue.Code == "account_scope_changed"
			}
			if point.TotalUSD != tc.total || len(point.BoundaryEvents) != tc.events || changed != tc.changed {
				t.Fatalf("unexpected transition: %+v", point)
			}
		})
	}
	// A later ordinary selection is a real boundary change.
	splitPrevious := map[uuid.UUID]previousAccount{earnID: {accountType: "earn", scopeVersion: 2, identityID: identityID, platform: "okx", value: decimal.MustParse("60")}}
	point := buildPoint(uuid.New(), uuid.New(), time.Now(), []AccountValuation{makeAccount(earnID, "earn", "60"), makeAccount(dualID, "dual_investment", "40")}, splitPrevious)
	if point.BoundaryContributionUSD != "40" {
		t.Fatalf("ordinary Dual opt-in must contribute 40: %+v", point)
	}
}
