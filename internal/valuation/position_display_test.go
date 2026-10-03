package valuation

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
)

func TestPositionSizeDirectionAndUnknowns(t *testing.T) {
	for _, tc := range []struct{ name, kind, unit, quantity, factor, mode, side, want, currency string }{
		{"linear perpetual contracts", adapters.KindLinearPerp, "contracts", "3", "0.01", "one_way", "short", "-0.03", "BTC"},
		{"one-way long", adapters.KindLinearPerp, "base", "0.2", "100", "one_way", "long", "0.2", "BTC"},
		{"linear delivery base not multiplied", adapters.KindLinearDeliv, "base", "2", "100", "hedged", "short", "2", "BTC"},
		{"inverse perpetual contracts", adapters.KindInversePerp, "contracts", "4", "100", "one_way", "short", "-400", "USD"},
		{"inverse delivery quote not multiplied", adapters.KindInverseDlv, "quote", "500", "100", "hedged", "long", "500", "USD"},
		{"unknown mode preserves direction and absolute size", adapters.KindLinearPerp, "base", "2", "", "", "short", "2", "BTC"},
		{"missing factor", adapters.KindInversePerp, "contracts", "4", "", "one_way", "long", "", "USD"},
		{"zero factor", adapters.KindLinearPerp, "contracts", "4", "0", "hedged", "long", "", "BTC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &AccountValuation{AccountID: uuid.New(), Freshness: FreshnessFresh, accountStructure: string(adapters.StructureBinanceClassic)}
			id := uuid.New()
			row, err := positionRow(account, id[:], "binance", "BTCUSD", tc.kind, nullString("BTC"), nullString("USD"), nullString("BTC"), sql.NullTime{}, nullString(tc.factor), "ref", tc.side, tc.quantity, tc.unit, sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{})
			if err != nil {
				t.Fatal(err)
			}
			completePosition(&row, account, positionMetadata{mode: nullString(tc.mode)})
			if stringValue(row.Size) != tc.want || stringValue(row.SizeSymbol) != tc.currency {
				t.Fatalf("Size=%v %v", row.Size, row.SizeSymbol)
			}
			if row.Quantity != tc.quantity || row.Side != tc.side {
				t.Fatal("source quantity or direction changed")
			}
			if tc.mode == "" && (len(row.Issues) == 0 || row.Issues[0].Code != "position_mode_unknown") {
				t.Fatal("missing mode was not named")
			}
		})
	}
}

func TestSettleMarginBasisConversionAndPortfolio(t *testing.T) {
	for _, tc := range []struct {
		name, mode, basis, from, to, mark string
		portfolio                         *bool
		want                              string
	}{
		{"native cross", "cross", "initial", "USDT", "USDT", "", boolPtr(false), "100"},
		{"stablecoin conversion", "cross", "initial", "USD", "USDC", "", boolPtr(false), "100"},
		{"inverse conversion", "cross", "initial", "USD", "BTC", "50000", boolPtr(false), "0.002"},
		{"missing price", "cross", "initial", "USD", "BTC", "", boolPtr(false), ""},
		{"zero price", "cross", "initial", "USD", "BTC", "0", boolPtr(false), ""},
		{"negative price", "cross", "initial", "USD", "BTC", "-2", boolPtr(false), ""},
		{"allocation", "isolated", "allocated", "BTC", "BTC", "", boolPtr(false), "100"},
		{"legacy isolated initial is not allocation", "isolated", "initial", "BTC", "BTC", "", boolPtr(false), ""},
		{"portfolio cross risk is not allocation", "cross", "initial", "USD", "USD", "", boolPtr(true), ""},
		{"portfolio isolated allocation", "isolated", "allocated", "USD", "USD", "", boolPtr(true), "100"},
		{"unknown account regime", "cross", "initial", "USD", "USD", "", nil, ""},
		{"unrelated currency", "cross", "initial", "EUR", "BTC", "50000", boolPtr(false), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := PositionRow{Margin: strPtr("100"), MarginMode: strPtr(tc.mode), MarginBasis: strPtr(tc.basis), MarginCurrency: strPtr(tc.from), SettlementSymbol: strPtr(tc.to), BaseSymbol: strPtr("BTC"), QuoteSymbol: strPtr("USD"), MarkPrice: nullDecimal(nullString(tc.mark)), PortfolioMargin: tc.portfolio}
			if got := stringValue(settlePositionMargin(&row)); got != tc.want {
				t.Fatalf("settle Margin=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestPortfolioUsesSourceReportMode(t *testing.T) {
	account := &AccountValuation{accountStructure: string(adapters.StructureBybitUTA2), sourceMarginMode: strPtr(string(adapters.MarginBybitPM))}
	if value := portfolioMargin(account); value == nil || !*value {
		t.Fatal("source Portfolio mode lost")
	}
	account.sourceMarginMode = nil
	if portfolioMargin(account) != nil {
		t.Fatal("unknown historical mode was guessed")
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func strPtr(value string) *string { return &value }
func boolPtr(value bool) *bool    { return &value }
