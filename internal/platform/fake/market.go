package fake

import (
	"context"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// CollectPrices supplies deterministic public prices independently of private key scenarios.
func (a *Adapter) CollectPrices(ctx context.Context, symbols []string) ([]adapters.Price, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	at := a.now().UTC()
	lookup := func(symbol, quote string) (adapters.Quote, bool) {
		price := DemoPrice(a.platform, symbol, at)
		return adapters.Quote{Price: price, SourceMarketRef: symbol + quote}, price != ""
	}
	return adapters.SymbolPrices(symbols, []string{"USD"}, lookup, at), nil
}
