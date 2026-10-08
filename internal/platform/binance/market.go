package binance

import (
	"context"
	"math/big"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// quotes are the spot quote coins prices are read in, in order of preference.
// Both value at 1 USD (ADR 0003).
var quotes = []string{"USDT", "USDC"}

// CollectPrices reads public market data without private credentials or account checks.
func (a *Adapter) CollectPrices(ctx context.Context, symbols []string) ([]adapters.Price, error) {
	prices, at := a.lastPrices(ctx, a.client(adapters.Credentials{}))
	return adapters.SymbolPrices(symbols, quotes, prices.quote, at), ctx.Err()
}

// lastPrices holds every spot last price by pair symbol, such as BTCUSDT.
type lastPrices map[string]string

// quote returns the last price of symbol in quote; it is the batch's adapters.QuoteLookup.
func (p lastPrices) quote(symbol, quote string) (string, bool) {
	price, ok := p[symbol+quote]
	return price, ok
}

// usd returns the USD value of one unit of symbol, for the Earn completeness check.
func (p lastPrices) usd(symbol string) (*big.Rat, bool) {
	return adapters.USDPrice(symbol, quotes, p.quote)
}

// lastPrices returns every spot last price and when it arrived. A failure yields no prices rather
// than failing the batch: valuation falls back to the latest price within 24 hours, and the Earn
// completeness check is skipped for coins without a price.
func (a *Adapter) lastPrices(ctx context.Context, c *client) (lastPrices, time.Time) {
	var rows []struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := c.call(ctx, epTickerPrice, nil, &rows); err != nil {
		return nil, time.Time{}
	}
	out := make(lastPrices, len(rows))
	for _, row := range rows {
		// A delisted pair stays listed with price 0, which is not a value.
		if price, ok := decimal.Parse(row.Price); ok && decimal.Valid(row.Price) && price.Sign() > 0 {
			out[row.Symbol] = row.Price
		}
	}
	return out, a.now()
}
