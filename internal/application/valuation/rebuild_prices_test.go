package valuation

import (
	"context"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

func TestHistoricalPriceSelection(t *testing.T) {
	at := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, symbol, want string
		prices             []sqlcgen.Price
	}{
		{"stablecoin without history", "USDT", "1", nil},
		{"unknown is not zero", "BTC", "", nil},
		{"platform before newer fallback", "BTC", "10", []sqlcgen.Price{
			{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: at.Add(-time.Hour), Price: "10", QuoteCanonicalAssetSymbol: "USD"},
			{Platform: "okx", CanonicalAssetSymbol: "BTC", ObservedAt: at, Price: "20", QuoteCanonicalAssetSymbol: "USDT"},
		}},
		{"expired platform uses fallback", "BTC", "20", []sqlcgen.Price{
			{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: at.Add(-25 * time.Hour), Price: "10", QuoteCanonicalAssetSymbol: "USD"},
			{Platform: "okx", CanonicalAssetSymbol: "BTC", ObservedAt: at.Add(-time.Hour), Price: "20", QuoteCanonicalAssetSymbol: "USDC"},
		}},
		{"exact age boundary", "BTC", "10", []sqlcgen.Price{
			{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: at.Add(-maxPriceAge), Price: "10", QuoteCanonicalAssetSymbol: "USD"},
		}},
		{"expired is unknown", "BTC", "", []sqlcgen.Price{
			{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: at.Add(-maxPriceAge - time.Microsecond), Price: "10", QuoteCanonicalAssetSymbol: "USD"},
		}},
		{"invalid latest quote does not reuse older quote", "BTC", "", []sqlcgen.Price{
			{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: at.Add(-time.Hour), Price: "10", QuoteCanonicalAssetSymbol: "USD"},
			{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: at, Price: "20", QuoteCanonicalAssetSymbol: "EUR"},
		}},
		{"future observation excluded", "BTC", "10", []sqlcgen.Price{
			{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: at, Price: "10", QuoteCanonicalAssetSymbol: "USD"},
			{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: at.Add(time.Microsecond), Price: "20", QuoteCanonicalAssetSymbol: "USD"},
		}},
		{"known zero retained", "BTC", "0", []sqlcgen.Price{
			{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: at, Price: "0", QuoteCanonicalAssetSymbol: "USD"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			book := newHistoricalPrices(nil, tc.prices)
			for range 2 {
				price, found, err := book.Lookup(context.Background(), "binance", tc.symbol, at)
				if err != nil || found != (tc.want != "") {
					t.Fatalf("found=%v error=%v", found, err)
				}
				if found && price.RatString() != tc.want {
					t.Fatalf("price %s, want %s", price, tc.want)
				}
			}
		})
	}
}

func TestManualHistoricalPricesRequireOwnPlatform(t *testing.T) {
	at := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		ownAt time.Time
		quote string
		want  bool
	}{
		{"exact expiry", at.Add(-maxPriceAge), "USD", true},
		{"expired", at.Add(-maxPriceAge - time.Microsecond), "USD", false},
		{"future", at.Add(time.Microsecond), "USD", false},
		{"invalid quote", at, "EUR", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			book := newHistoricalPrices(nil, []sqlcgen.Price{{Platform: "binance", CanonicalAssetSymbol: "BTC", ObservedAt: tc.ownAt, Price: "10", QuoteCanonicalAssetSymbol: tc.quote}, {Platform: "okx", CanonicalAssetSymbol: "BTC", ObservedAt: at, Price: "20", QuoteCanonicalAssetSymbol: "USD"}})
			_, found, err := book.LookupPlatform(context.Background(), "binance", "BTC", at)
			if err != nil || found != tc.want {
				t.Fatalf("qualified price found=%v err=%v", found, err)
			}
		})
	}
}
