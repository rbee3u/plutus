package adapters

import "testing"

func TestEarnDetailsMatchBalancesExactly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		balances []Balance
		valid    bool
	}{
		{"matching", []Balance{{Symbol: "BTC", Equity: "1"}, {Symbol: "ETH", Equity: "2"}}, true},
		{"reordered and equivalent", []Balance{{Symbol: "eth", Equity: "2.00"}, {Symbol: "btc", Equity: "1.0"}}, true},
		{"missing asset", []Balance{{Symbol: "BTC", Equity: "1"}}, false},
		{"extra asset", []Balance{{Symbol: "BTC", Equity: "1"}, {Symbol: "ETH", Equity: "2"}, {Symbol: "USDT", Equity: "3"}}, false},
		{"duplicate masks missing asset", []Balance{{Symbol: "BTC", Equity: "1"}, {Symbol: "btc", Equity: "1"}}, false},
		{"invalid amount", []Balance{{Symbol: "BTC", Equity: "invalid"}, {Symbol: "ETH", Equity: "2"}}, false},
		{"mismatched amount", []Balance{{Symbol: "BTC", Equity: "9"}, {Symbol: "ETH", Equity: "2"}}, false},
		{"negative amount", []Balance{{Symbol: "BTC", Equity: "-1"}, {Symbol: "ETH", Equity: "2"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewEarnPosition("product:pair", "flexible", "product", "total")
			p.Assets = []EarnAmount{{Symbol: "BTC", Amount: "1"}, {Symbol: "ETH", Amount: "2"}}
			err := CheckEarnPositions([]EarnPosition{p}, tc.balances)
			if (err == nil) != tc.valid {
				t.Fatalf("CheckEarnPositions = %v, want valid %v", err, tc.valid)
			}
		})
	}
}

func TestEarnOptionalMetadataPreservesSignedReportedValues(t *testing.T) {
	p := NewEarnPosition("product:USDT", "flexible", "product", "principal")
	p.AddReward("USDT", "-0.25", "reported")
	if len(p.Rewards) != 1 || p.Rewards[0].Amount != "-0.25" || len(p.Problems) != 0 {
		t.Fatalf("signed source reward was lost: %+v", p)
	}
}
