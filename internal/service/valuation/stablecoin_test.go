package valuation

import "testing"

func TestStablecoinClassification(t *testing.T) {
	for _, symbol := range []string{
		"BUSD", "DAI", "FDUSD", "FRAX", "GUSD", "LUSD", "PYUSD", "RLUSD",
		"TUSD", "U", "USD", "USD1", "USDC", "USDD", "USDE", "USDG", "USDP",
		"USDS", "USDT", "USDTB",
	} {
		if !Stablecoin(symbol) {
			t.Fatalf("Stablecoin(%q) = false", symbol)
		}
	}
	for _, symbol := range []string{"", "BTC", "ETH", "USDC.E", "SUSDE", "EURC", "usdt"} {
		if Stablecoin(symbol) {
			t.Fatalf("Stablecoin(%q) = true", symbol)
		}
	}
}
