package adapters

import (
	"testing"
	"time"
)

func TestSymbolPricesSkipsNonpositivePreferredQuote(t *testing.T) {
	prices := SymbolPrices([]string{"BTC"}, []string{"USD", "USDT", "USDC"}, func(_, quote string) (Quote, bool) {
		values := map[string]string{"USD": "0", "USDT": "-2", "USDC": "123"}
		return Quote{Price: values[quote], SourceMarketRef: "BTC" + quote}, true
	}, time.Now())
	if len(prices) != 1 || prices[0].Price != "123" || prices[0].QuoteCanonicalAssetSymbol != "USDC" {
		t.Fatalf("invalid preferred quote blocked fallback: %+v", prices)
	}
}
