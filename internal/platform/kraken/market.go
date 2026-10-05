package kraken

import (
	"context"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// CollectPrices reads public market data without private credentials or account checks.
func (a *Adapter) CollectPrices(ctx context.Context, symbols []string) ([]adapters.Price, error) {
	c := a.client(adapters.Credentials{})
	assets, err := a.assetNames(ctx, c)
	if err != nil {
		return nil, err
	}
	prices := a.spotPrices(ctx, c, assets)
	return adapters.SymbolPrices(symbols, priceQuotes, prices.lookup, a.now()), ctx.Err()
}
