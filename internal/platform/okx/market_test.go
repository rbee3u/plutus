package okx

import "testing"

func TestPublicManualPricesWithoutCredentials(t *testing.T) {
	r, a := newReplay(t)
	prices, err := a.CollectPrices(t.Context(), []string{"BTC", "BTC", "USD", "UNLISTED"})
	if err != nil || len(prices) != 1 || prices[0].CanonicalAssetSymbol != "BTC" {
		t.Fatalf("public prices %+v %v", prices, err)
	}
	for _, ep := range []endpoint{epConfig, epBalance, epPositions, epFunding, epSavings, epStaking, epDCD, epValuation} {
		if r.count(ep) > 0 {
			t.Fatal("private call", ep)
		}
	}
}

func TestNonzeroTradingBalanceRequiresCurrency(t *testing.T) {
	for _, row := range []tradingBalance{{Eq: "1"}, {Eq: "0", Liab: "1"}, {Eq: "0", AvailBal: "1"}, {Ccy: " ", Eq: "-1"}} {
		if _, err := mapTradingBalances([]tradingBalance{row}); err == nil {
			t.Fatalf("nonzero balance silently omitted: %+v", row)
		}
	}
	if rows, err := mapTradingBalances([]tradingBalance{{Eq: "0"}}); err != nil || len(rows) != 0 {
		t.Fatalf("empty zero row: %+v %v", rows, err)
	}
}
