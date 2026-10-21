package kraken

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// priceQuotes are the Spot quotes a balance is priced in, in order of
// preference: USD, then the two stablecoins ADR 0003 values at 1 USD.
var priceQuotes = []string{"USD", "USDT", "USDC"}

// CollectPrices reads public market data without private credentials or account checks.
func (a *Adapter) CollectPrices(ctx context.Context, symbols []string) ([]adapter.Price, error) {
	c := a.client(adapter.Credentials{})
	assets, err := readAssetNames(ctx, c)
	if err != nil {
		return nil, err
	}
	prices := readSpotPrices(ctx, c, assets)
	return adapter.SymbolPrices(symbols, priceQuotes, prices.lookup, a.now()), ctx.Err()
}

// assetNames maps Kraken asset codes such as XXBT and ZUSD to the public names assets lists for
// them (XBT, USD).
type assetNames map[string]string

// aliases are the legacy Kraken names that differ from the symbols Plutus
// merges by (ADR 0003).
var aliases = map[string]string{"XBT": "BTC", "XXBT": "BTC", "XDG": "DOGE", "XXDG": "DOGE", "ZUSD": "USD"}

// readAssetNames reads the public asset list.
func readAssetNames(ctx context.Context, c *client) (assetNames, error) {
	var rows map[string]struct {
		Altname string `json:"altname"`
	}
	if err := c.call(ctx, epAssets, nil, &rows); err != nil {
		return nil, err
	}
	out := make(assetNames, len(rows))
	for code, row := range rows {
		out[code] = row.Altname
	}
	return out, nil
}

// symbol normalizes a Kraken asset code: its public name when assets lists the code, then the
// aliases above. Genuine suffixes such as .T (tokenized assets) are kept.
func (n assetNames) symbol(where, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", adapter.Malformed(where, "an asset code")
	}
	if name := n[code]; name != "" {
		code = name
	}
	code = strings.ToUpper(code)
	if alias, ok := aliases[code]; ok {
		return alias, nil
	}
	return code, nil
}

// mainSymbol normalizes a BalanceEx code and reports whether the row belongs to main.
func (n assetNames) mainSymbol(where, code string) (string, bool, error) {
	for _, suffix := range allocatedSuffixes {
		if strings.HasSuffix(code, suffix) {
			return "", false, nil
		}
	}
	symbol, err := n.symbol(where, strings.TrimSuffix(code, ".F"))
	return symbol, true, err
}

// spotPrices holds the last Spot price of each asset per quote.
type spotPrices map[string]map[string]adapter.Quote

// lookup is the adapter.QuoteLookup of the Spot prices.
func (p spotPrices) lookup(symbol, quote string) (adapter.Quote, bool) {
	price, ok := p[symbol][quote]
	return price, ok
}

// readSpotPrices returns the last Spot price of every asset with a USD, USDT, or USDC pair. A failed
// call yields no prices rather than failing the batch: valuation falls back to the latest price
// within 24 hours.
func readSpotPrices(ctx context.Context, c *client, assets assetNames) spotPrices {
	var pairs map[string]struct {
		Base  string `json:"base"`
		Quote string `json:"quote"`
	}
	if c.call(ctx, epAssetPairs, nil, &pairs) != nil {
		return nil
	}
	var tickers map[string]struct {
		Close []number `json:"c"`
	}
	if c.call(ctx, epSpotTicker, nil, &tickers) != nil {
		return nil
	}
	out := spotPrices{}
	for _, name := range slices.Sorted(maps.Keys(pairs)) {
		base, err := assets.symbol("", pairs[name].Base)
		if err != nil {
			continue
		}
		quote, err := assets.symbol("", pairs[name].Quote)
		if err != nil || !slices.Contains(priceQuotes, quote) {
			continue
		}
		ticker, ok := tickers[name]
		if !ok || len(ticker.Close) == 0 || out[base][quote].Price != "" {
			continue
		}
		if last, ok := decimal.Parse(string(ticker.Close[0])); ok && decimal.Valid(string(ticker.Close[0])) && last.Sign() > 0 {
			if out[base] == nil {
				out[base] = map[string]adapter.Quote{}
			}
			out[base][quote] = adapter.Quote{Price: string(ticker.Close[0]), SourceMarketRef: name}
		}
	}
	return out
}
