package binance

import (
	"context"

	"github.com/rbee3u/plutus/internal/adapters"
)

// CollectPrices reads public market data without private credentials or account checks.
func (a *Adapter) CollectPrices(ctx context.Context, symbols []string) ([]adapters.Price, error) {
	prices, at := a.lastPrices(ctx, a.client(adapters.Credentials{}))
	return adapters.SymbolPrices(symbols, quotes, prices.quote, at), ctx.Err()
}
