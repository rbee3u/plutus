package adapters

import "testing"

func TestEarnDetailsRejectInvalidOrMismatchedBalance(t *testing.T) {
	p := NewEarnPosition("product:USDT", "flexible", "product", "total")
	p.Assets = append(p.Assets, EarnAmount{Symbol: "USDT", Amount: "10"})
	for _, amount := range []string{"invalid", "9", "-10"} {
		if err := CheckEarnPositions([]EarnPosition{p}, []Balance{{Symbol: "USDT", Equity: amount}}); err == nil {
			t.Fatalf("accepted invalid or mismatched balance %q", amount)
		}
	}
	if err := CheckEarnPositions([]EarnPosition{p}, []Balance{{Symbol: "USDT", Equity: "10"}}); err != nil {
		t.Fatal(err)
	}
}

func TestEarnOptionalMetadataPreservesSignedReportedValues(t *testing.T) {
	p := NewEarnPosition("product:USDT", "flexible", "product", "principal")
	p.AddReward("USDT", "-0.25", "reported")
	if len(p.Rewards) != 1 || p.Rewards[0].Amount != "-0.25" || len(p.Problems) != 0 {
		t.Fatalf("signed source reward was lost: %+v", p)
	}
}
