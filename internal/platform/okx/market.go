package okx

import (
	"context"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// CollectPrices reads public market data without private credentials or account checks.
func (a *Adapter) CollectPrices(ctx context.Context, symbols []string) ([]adapters.Price, error) {
	lookup, at := a.tickers(ctx, a.client(adapters.Credentials{}))
	return adapters.SymbolPrices(symbols, quotes, lookup, at), ctx.Err()
}
