package seeds

import (
	"fmt"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func validateReport(r *adapters.AccountReport) error {
	if r.Status != adapters.StatusSucceeded || r.CollectedAt.IsZero() {
		return fmt.Errorf("successful report needs status and observation time")
	}
	seen := map[string]bool{}
	for _, v := range r.Balances {
		if v.AssetSymbol == "" || seen[v.AssetSymbol] || !decimal.Valid(v.Equity) {
			return fmt.Errorf("invalid or duplicate Balance")
		}
		seen[v.AssetSymbol] = true
	}
	if r.AccountType == "earn" {
		if err := adapters.CheckEarnPositions(r.EarnPositions, r.Balances); err != nil {
			return err
		}
	} else if len(r.EarnPositions) != 0 {
		return fmt.Errorf("Earn descriptions require an Earn Account")
	}
	for _, p := range r.Positions {
		if p.PositionKey == "" || p.FuturesContract.Symbol == "" || !decimal.Valid(p.SourceQuantity) || decimal.ParseOrZero(p.SourceQuantity).Sign() <= 0 {
			return fmt.Errorf("invalid Position identity or quantity")
		}
		if (p.FuturesContract.Kind == adapters.KindLinearDelivery || p.FuturesContract.Kind == adapters.KindInverseDelivery) && p.FuturesContract.ExpiryAt == nil {
			return fmt.Errorf("delivery FuturesContract requires expiry")
		}
	}
	for _, p := range r.DualPositions {
		if p.PositionKey == "" || !decimal.Valid(p.Principal) || decimal.ParseOrZero(p.Principal).Sign() <= 0 {
			return fmt.Errorf("invalid Dual subscription")
		}
	}
	return nil
}
