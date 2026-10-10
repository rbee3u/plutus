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
