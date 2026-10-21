package financial

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

// TestPositionNotionalPerContractKind pins the contract-size convention: a linear contract's face
// value is a quantity of the base asset and needs the mark price, while an inverse contract's face
// value is already the USD notional of one contract.
func TestPositionNotionalPerContractKind(t *testing.T) {
	t.Parallel()
	futuresContractID := uuid.Must(uuid.NewV7())
	for _, tc := range []struct {
		name         string
		kind         string
		quote        string
		unit         string
		quantity     string
		contractSize string
		mark         string
		want         string
	}{
		// Binance COIN-M: one DOGEUSD_PERP contract is 10 USD regardless of the DOGE price.
		{"inverse perpetual", adapter.KindInversePerpetual, "USD", "contracts", "1", "10", "0.0934", "10"},
		{"inverse delivery", adapter.KindInverseDelivery, "USD", "contracts", "3", "100", "62000", "300"},
		{"inverse without a mark price", adapter.KindInversePerpetual, "USD", "contracts", "2", "10", "", "20"},
		{"short inverse position", adapter.KindInversePerpetual, "USD", "contracts", "-4", "10", "0.0934", "40"},
		// OKX linear swap: one DOGE-USDT-SWAP contract is 1000 DOGE, valued at the mark price.
		{"linear perpetual", adapter.KindLinearPerpetual, "USDT", "contracts", "2", "1000", "0.0934", "186.8"},
		{"linear delivery", adapter.KindLinearDelivery, "USDT", "contracts", "1", "0.01", "62000", "620"},
		{"linear without a mark price", adapter.KindLinearPerpetual, "USDT", "contracts", "2", "1000", "", ""},
		{"linear without a USD quote", adapter.KindLinearPerpetual, "BTC", "contracts", "2", "1000", "0.0934", ""},
		{"missing quote", adapter.KindInversePerpetual, "", "contracts", "2", "100", "50000", ""},
		{"zero mark", adapter.KindLinearPerpetual, "USDT", "base", "2", "", "0", ""},
		{"negative mark", adapter.KindLinearPerpetual, "USDT", "base", "2", "", "-2", ""},
		// Binance USDⓈ-M reports base units instead of contracts.
		{"base units", adapter.KindLinearPerpetual, "USDT", "base", "0.5", "", "62000", "31000"},
		{"inverse quote units", adapter.KindInversePerpetual, "USD", "quote", "150", "100", "", "150"},
		{"incompatible linear quote units", adapter.KindLinearPerpetual, "USDT", "quote", "150", "", "62000", ""},
		{"incompatible inverse base units", adapter.KindInversePerpetual, "USD", "base", "2", "", "62000", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			account := &valuation.AccountValuation{AccountID: uuid.Must(uuid.NewV7()), Freshness: valuation.FreshnessFresh}
			row, err := positionRow(account, &sqlcgen.ListPositionsByReportRow{
				FuturesContractID:     futuresContractID[:],
				Platform:              string(adapter.PlatformBinance),
				FuturesSymbol:         "SYMBOL",
				Kind:                  tc.kind,
				BaseAssetSymbol:       nullString("BASE"),
				QuoteAssetSymbol:      nullString(tc.quote),
				SettlementAssetSymbol: nullString("BASE"),
				ContractSize:          nullString(tc.contractSize),
				PositionKey:           "ref",
				Side:                  adapter.SideLong,
				SourceQuantity:        tc.quantity,
				SourceQuantityUnit:    tc.unit,
				MarkPrice:             nullString(tc.mark),
			})
			if err != nil {
				t.Fatalf("positionRow: %v", err)
			}
			got := ""
			if row.NotionalUSD != nil {
				got = *row.NotionalUSD
			}
			if got != tc.want {
				t.Fatalf("notional %q, want %q", got, tc.want)
			}
		})
	}
}
