package seeds

import (
	"fmt"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func validateReport(r *adapters.AccountReport) error {
	if r.Status != adapters.StatusOK || r.ObservedAt.IsZero() {
		return fmt.Errorf("successful report needs status and observation time")
	}
	seen := map[string]bool{}
	for _, v := range r.Balances {
		if v.Symbol == "" || seen[v.Symbol] || !decimal.Valid(v.Equity) {
			return fmt.Errorf("invalid or duplicate Balance")
		}
		seen[v.Symbol] = true
	}
	if r.EarnDetailsCollected {
		if err := adapters.CheckEarnPositions(r.EarnPositions, r.Balances); err != nil {
			return err
		}
	} else if len(r.EarnPositions) != 0 {
		return fmt.Errorf("Earn descriptions must be marked collected")
	}
	for _, p := range r.Positions {
		if p.PlatformPositionRef == "" || p.Instrument.Symbol == "" || !decimal.Valid(p.Quantity) || decimal.MustParse(p.Quantity).Sign() <= 0 {
			return fmt.Errorf("invalid Position identity or quantity")
		}
		if (p.Instrument.Kind == adapters.KindLinearDeliv || p.Instrument.Kind == adapters.KindInverseDlv) && p.Instrument.ExpiryAt == nil {
			return fmt.Errorf("delivery Instrument requires expiry")
		}
	}
	for _, p := range r.DualPositions {
		if p.PlatformPositionRef == "" || !decimal.Valid(p.Principal) || decimal.MustParse(p.Principal).Sign() <= 0 {
			return fmt.Errorf("invalid Dual subscription")
		}
	}
	return nil
}
