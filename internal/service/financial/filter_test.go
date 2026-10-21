package financial

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/service/valuation"
)

func TestSymbolSubstringSearch(t *testing.T) {
	for _, test := range []struct {
		name, query string
		symbols     []string
		want        bool
	}{
		{"native contract", "btc", []string{"BTC-USDT-SWAP"}, true},
		{"mapped asset", "bTc", []string{"XBT", "BTC"}, true},
		{"quote currency", "sDt", []string{"BTC", "USDT"}, true},
		{"non-ASCII", "牛来", []string{"牛来USDT"}, true},
		{"empty optional terms", "BTC", []string{"USDT", "", ""}, false},
		{"literal wildcard", "BTC%", []string{"BTCUSDT"}, false},
		{"literal underscore", "BTC_", []string{"BTC-USD"}, false},
		{"surrounding whitespace", "  btc ", []string{"BTCUSDT"}, true},
		{"empty query", "  ", nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := matchesSymbol(test.query, test.symbols...); got != test.want {
				t.Fatalf("matchesSymbol(%q, %v) = %v, want %v", test.query, test.symbols, got, test.want)
			}
		})
	}
}

func TestReportCoverageSeparatesFiltersFreshnessAndMissingReports(t *testing.T) {
	one, two, report := uuid.New(), uuid.New(), uuid.New()
	accounts := []valuation.AccountValuation{
		{AccountID: one, IdentityID: one, AccountType: "earn", Platform: "binance", SourceReportID: &report, Freshness: valuation.FreshnessStale},
		{AccountID: two, IdentityID: two, AccountType: "earn", Platform: "okx"},
	}
	for _, tc := range []struct {
		name, kind         string
		filter             ListFilter
		hasPoint, complete bool
		missing            int
	}{
		{"all", "earn", ListFilter{}, true, false, 1},
		{"stale remains readable", "earn", ListFilter{Platform: "binance"}, true, true, 0},
		{"identity scope", "earn", ListFilter{IdentityID: &two}, true, false, 1},
		{"search cannot prove absence", "earn", ListFilter{Symbol: "no-match"}, true, false, 1},
		{"other holding kind", "dual_investment", ListFilter{}, true, true, 0},
		{"no point", "earn", ListFilter{Platform: "binance"}, false, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := reportCoverage(accounts, &tc.filter, tc.kind, tc.hasPoint)
			if got.Complete != tc.complete || len(got.MissingAccountIDs) != tc.missing {
				t.Fatalf("coverage: %+v", got)
			}
		})
	}
}
