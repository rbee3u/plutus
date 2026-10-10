package valuation

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

func TestHistoricalPriceEligibilityPrecedesSelection(t *testing.T) {
	at := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	observation := func(platform, price, quote string, priceAt, retrieved time.Time) sqlcgen.PriceObservation {
		id := uuid.New()
		return sqlcgen.PriceObservation{ID: id[:], Platform: platform, RequestedCanonicalAssetSymbol: "BTC", Price: price, QuoteCanonicalAssetSymbol: quote, PriceAt: sql.NullTime{Time: priceAt, Valid: true}, RetrievedAt: retrieved}
	}
	valid := observation("binance", "10", "USD", at.Add(-time.Hour), at.Add(-time.Hour))
	for _, tc := range []struct {
		name, symbol, want string
		rows               []sqlcgen.PriceObservation
	}{
		{"parity", "USDT", "1", nil},
		{"unknown", "BTC", "", nil},
		{"own platform only", "BTC", "10", []sqlcgen.PriceObservation{valid, observation("okx", "20", "USD", at, at)}},
		{"other platform cannot rescue missing", "BTC", "", []sqlcgen.PriceObservation{observation("okx", "20", "USD", at, at)}},
		{"exact age inclusive", "BTC", "10", []sqlcgen.PriceObservation{observation("binance", "10", "USDC", at.Add(-(24 * time.Hour)), at)}},
		{"expired", "BTC", "", []sqlcgen.PriceObservation{observation("binance", "10", "USD", at.Add(-(24*time.Hour)-time.Microsecond), at)}},
		{"future source", "BTC", "10", []sqlcgen.PriceObservation{valid, observation("binance", "20", "USD", at.Add(time.Second), at)}},
		{"not yet known", "BTC", "10", []sqlcgen.PriceObservation{valid, observation("binance", "20", "USD", at, at.Add(time.Second))}},
		{"zero price invalid", "BTC", "", []sqlcgen.PriceObservation{observation("binance", "0", "USD", at, at)}},
		{"latest retrieval wins", "BTC", "20", []sqlcgen.PriceObservation{valid, observation("binance", "20", "USD", valid.PriceAt.Time, at)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPointFixture(t)
			for _, row := range tc.rows {
				id := uuid.New()
				row.ID = id[:]
				if row.Price == "0" {
					continue
				}
				f.exec("INSERT INTO price_observations(id,portfolio_id,platform,requested_canonical_asset_symbol,source_market_ref,quote_canonical_asset_symbol,price,retrieved_at,source_observed_at) VALUES(?,?,?,?,'fixture',?,?,?,?)", row.ID, f.portfolio[:], row.Platform, row.RequestedCanonicalAssetSymbol, row.QuoteCanonicalAssetSymbol, row.Price, row.RetrievedAt, row.PriceAt)
			}
			result, err := NewPriceBook(sqlcgen.New(f.db), f.portfolio).Resolve(f.ctx, "binance", tc.symbol, at)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if result.Value != nil {
					t.Fatalf("unexpected price: %+v", result)
				}
				return
			}
			if result.Value == nil || result.Value.RatString() != tc.want {
				t.Fatalf("price: %+v, want %s", result, tc.want)
			}
			if tc.symbol == "BTC" && result.ObservationID == nil {
				t.Fatal("missing provenance")
			}
		})
	}
}

func TestPriceObservationRejectsNonUSDQuote(t *testing.T) {
	f := newPointFixture(t)
	id := uuid.New()
	_, err := f.db.ExecContext(f.ctx, "INSERT INTO price_observations(id,portfolio_id,platform,requested_canonical_asset_symbol,source_market_ref,quote_canonical_asset_symbol,price,retrieved_at) VALUES(?,?,'binance','BTC','fixture','EUR',20,UTC_TIMESTAMP(6))", id[:], f.portfolio[:])
	if err == nil || !strings.Contains(err.Error(), "chk_price_observations_usd_quote") {
		t.Fatalf("non-USD quote accepted or failed for another reason: %v", err)
	}
}
