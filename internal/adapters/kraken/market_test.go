package kraken

import "testing"

func TestPublicManualPricesWithoutCredentials(t *testing.T) {
	r := newReplay(t, "", nil)
	prices, err := r.adapter.CollectPrices(t.Context(), []string{"BTC", "BTC", "USD", "UNLISTED"})
	if err != nil || len(prices) != 1 || prices[0].AssetSymbol != "BTC" {
		t.Fatalf("public prices %+v %v", prices, err)
	}
	if r.reached(epBalancesSnapshot.String()) || r.reached(epBalanceEx.String()) {
		t.Fatal("private preflight reached")
	}
}
