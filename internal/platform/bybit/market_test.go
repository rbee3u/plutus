package bybit

import "testing"

func TestPublicManualPricesWithoutCredentials(t *testing.T) {
	r := newReplay(t, "", nil)
	prices, err := r.adapter.CollectPrices(t.Context(), []string{"BTC", "BTC", "USD", "UNLISTED"})
	if err != nil || len(prices) != 1 || prices[0].CanonicalAssetSymbol != "BTC" {
		t.Fatalf("public prices %+v %v", prices, err)
	}
	if r.count(epAccountInfo.path) != 0 || r.count(epWallet.path) != 0 {
		t.Fatal("private preflight reached")
	}
}
