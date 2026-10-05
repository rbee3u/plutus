package binance

import "testing"

func TestPublicManualPricesWithoutCredentials(t *testing.T) {
	r, a := newReplay(t, "")
	prices, err := a.CollectPrices(t.Context(), []string{"BTC", "BTC", "USD", "UNLISTED"})
	if err != nil || len(prices) != 1 || prices[0].AssetSymbol != "BTC" {
		t.Fatalf("public prices %+v %v", prices, err)
	}
	for _, ep := range endpoints {
		if ep.signed && r.count(ep) > 0 {
			t.Fatalf("private endpoint %v", ep)
		}
	}
}
