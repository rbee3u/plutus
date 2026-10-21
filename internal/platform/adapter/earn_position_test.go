package adapter

import (
	"strings"
	"testing"
)

func TestEarnDetailsMatchBalancesExactly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		balances []Balance
		valid    bool
	}{
		{"matching", []Balance{{AssetSymbol: "BTC", Equity: "1"}, {AssetSymbol: "ETH", Equity: "2"}}, true},
		{"reordered and equivalent", []Balance{{AssetSymbol: "eth", Equity: "2.00"}, {AssetSymbol: "btc", Equity: "1.0"}}, true},
		{"missing asset", []Balance{{AssetSymbol: "BTC", Equity: "1"}}, false},
		{"extran asset", []Balance{{AssetSymbol: "BTC", Equity: "1"}, {AssetSymbol: "ETH", Equity: "2"}, {AssetSymbol: "USDT", Equity: "3"}}, false},
		{"duplicate masks missing asset", []Balance{{AssetSymbol: "BTC", Equity: "1"}, {AssetSymbol: "btc", Equity: "1"}}, false},
		{"invalid amount", []Balance{{AssetSymbol: "BTC", Equity: "invalid"}, {AssetSymbol: "ETH", Equity: "2"}}, false},
		{"mismatched amount", []Balance{{AssetSymbol: "BTC", Equity: "9"}, {AssetSymbol: "ETH", Equity: "2"}}, false},
		{"negative amount", []Balance{{AssetSymbol: "BTC", Equity: "-1"}, {AssetSymbol: "ETH", Equity: "2"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewEarnPosition("product:pair", "flexible", "product", "total")
			p.Assets = []EarnAmount{{AssetSymbol: "BTC", Amount: "1"}, {AssetSymbol: "ETH", Amount: "2"}}
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

func TestEarnOptionalTextLimitsPreserveAmounts(t *testing.T) {
	for _, length := range []int{255, 256} {
		p := NewEarnPosition("product:BTC", "flexible", "product", "principal")
		p.Assets = []EarnAmount{{AssetSymbol: "BTC", Amount: "1"}}
		p.ProductName = new(strings.Repeat("币", length))
		p.Status = new(strings.Repeat("x", length-191))
		if err := NormalizeEarnPosition(&p); err != nil {
			t.Fatal(err)
		}
		if p.Assets[0].Amount != "1" {
			t.Fatal("optional text changed amounts")
		}
		if length == 255 && (p.ProductName == nil || p.Status == nil || len(p.Problems) != 0) {
			t.Fatalf("valid limits lost: %+v", p)
		}
		if length == 256 && (p.ProductName != nil || p.Status != nil || len(p.Problems) != 2) {
			t.Fatalf("invalid metadata survived: %+v", p)
		}
	}
}
