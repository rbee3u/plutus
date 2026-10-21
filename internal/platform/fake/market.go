package fake

import (
	"context"

	"github.com/rbee3u/plutus/internal/platform/adapter"
)

// CollectPrices supplies deterministic public prices independently of private key scenarios.
func (a *Adapter) CollectPrices(ctx context.Context, symbols []string) ([]adapter.Price, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	at := a.now().UTC()
	lookup := func(symbol, quote string) (adapter.Quote, bool) {
		price := DemoPrice(a.platform, symbol, at)
		return adapter.Quote{Price: price, SourceMarketRef: symbol + quote}, price != ""
	}
	return adapter.SymbolPrices(symbols, []string{"USD"}, lookup, at), nil
}
