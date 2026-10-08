package bybit

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// quotes are the spot quotes a coin is priced in, in order of preference. Both
// value at 1 USD (ADR 0003).
var quotes = []string{"USDT", "USDC"}

// CollectPrices reads public market data without private credentials or account checks.
func (a *Adapter) CollectPrices(ctx context.Context, symbols []string) ([]adapters.Price, error) {
	prices, at := a.tickers(ctx, a.client(adapters.Credentials{}))
	return adapters.SymbolPrices(symbols, quotes, prices.lookup, at), ctx.Err()
}

// spotPrices are Bybit spot last prices by pair symbol, such as BTCUSDT.
type spotPrices map[string]string

func (p spotPrices) lookup(symbol, quote string) (string, bool) {
	price, ok := p[symbol+quote]
	return price, ok
}

// usd returns the USD value of one unit of symbol, for the Earn check.
func (p spotPrices) usd(symbol string) (*big.Rat, bool) {
	return adapters.USDPrice(symbol, quotes, p.lookup)
}

// tickers reads every spot last price in one call. A failure yields no prices rather than failing
// the batch: valuation falls back to the latest price within 24 hours.
func (a *Adapter) tickers(ctx context.Context, c *client) (spotPrices, time.Time) {
	var result struct {
		List []struct {
			Symbol    string `json:"symbol"`
			LastPrice string `json:"lastPrice"`
		} `json:"list"`
	}
	if err := c.call(ctx, epTickers, nil, &result); err != nil {
		return spotPrices{}, time.Time{}
	}
	prices := make(spotPrices, len(result.List))
	for _, row := range result.List {
		if price, ok := decimal.Parse(row.LastPrice); ok && decimal.Valid(row.LastPrice) && price.Sign() > 0 {
			prices[strings.TrimSpace(row.Symbol)] = decimal.Compact(strings.TrimSpace(row.LastPrice))
		}
	}
	return prices, a.now()
}
