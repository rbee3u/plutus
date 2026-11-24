package valuation

// stablecoinSymbols is the maintained set of USD-pegged canonical assets that
// exposure value reports as stablecoins. Classification is explicit by
// canonical symbol; a price near one never classifies an asset. USD, USDT, and
// USDC value at exactly one USD, while the other listed assets need a
// reference price.
//
// Keep this set in sync with docs/domain-rules.md#exposure-usd-composition.
var stablecoinSymbols = map[string]bool{
	"BUSD":  true,
	"DAI":   true,
	"FDUSD": true,
	"FRAX":  true,
	"GUSD":  true,
	"LUSD":  true,
	"PYUSD": true,
	"RLUSD": true,
	"TUSD":  true,
	"U":     true,
	"USD":   true,
	"USD1":  true,
	"USDC":  true,
	"USDD":  true,
	"USDE":  true,
	"USDG":  true,
	"USDP":  true,
	"USDS":  true,
	"USDT":  true,
	"USDTB": true,
}

// Stablecoin reports whether a canonical asset symbol belongs to the maintained
// USD-pegged stablecoin set.
func Stablecoin(symbol string) bool {
	return stablecoinSymbols[symbol]
}
