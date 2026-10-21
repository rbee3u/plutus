package okx

import (
	"context"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// quotes are the spot quote coins prices are read in, in order of preference.
// Both value at 1 USD (ADR 0003).
var quotes = []string{"USDT", "USDC"}

// CollectPrices reads public market data without private credentials or account checks.
func (a *Adapter) CollectPrices(ctx context.Context, symbols []string) ([]adapter.Price, error) {
	lookup, at := a.tickers(ctx, a.client(adapter.Credentials{}))
	return adapter.SymbolPrices(symbols, quotes, lookup, at), ctx.Err()
}

// tickers returns a lookup of every spot last price and the time it was read. A failure yields
// no prices rather than failing the batch: valuation falls back to the latest price within 24
// hours, and the earn check is skipped.
func (a *Adapter) tickers(ctx context.Context, c *client) (adapter.QuoteLookup, time.Time) {
	var rows []struct {
		InstID string `json:"instId"`
		Last   string `json:"last"`
	}
	if err := c.call(ctx, epTickers, &rows); err != nil {
		return nil, time.Time{}
	}
	last := make(map[string]string, len(rows))
	for _, row := range rows {
		if value, ok := decimal.Parse(row.Last); decimal.Valid(row.Last) && ok && value.Sign() > 0 {
			last[row.InstID] = row.Last
		}
	}
	return spotLookup(last), a.now()
}

// spotLookup prices a coin through its <symbol>-<quote> spot pair.
func spotLookup(last map[string]string) adapter.QuoteLookup {
	return func(symbol, quote string) (adapter.Quote, bool) {
		price, ok := last[symbol+"-"+quote]
		return adapter.Quote{Price: price, SourceMarketRef: symbol + "-" + quote}, ok
	}
}
