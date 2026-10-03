package valuation

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
)

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

// TestPositionNotionalPerContractKind pins the contract-size convention: a linear contract's face
// value is a quantity of the base Asset and needs the mark price, while an inverse contract's face
// value is already the USD notional of one contract.
func TestPositionNotionalPerContractKind(t *testing.T) {
	t.Parallel()
	instrumentID := uuid.Must(uuid.NewV7())
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
		{"inverse perpetual", adapters.KindInversePerp, "USD", "contracts", "1", "10", "0.0934", "10"},
		{"inverse delivery", adapters.KindInverseDlv, "USD", "contracts", "3", "100", "62000", "300"},
		{"inverse without a mark price", adapters.KindInversePerp, "USD", "contracts", "2", "10", "", "20"},
		{"short inverse position", adapters.KindInversePerp, "USD", "contracts", "-4", "10", "0.0934", "40"},
		// OKX linear swap: one DOGE-USDT-SWAP contract is 1000 DOGE, valued at the mark price.
		{"linear perpetual", adapters.KindLinearPerp, "USDT", "contracts", "2", "1000", "0.0934", "186.8"},
		{"linear delivery", adapters.KindLinearDeliv, "USDT", "contracts", "1", "0.01", "62000", "620"},
		{"linear without a mark price", adapters.KindLinearPerp, "USDT", "contracts", "2", "1000", "", ""},
		{"linear without a USD quote", adapters.KindLinearPerp, "BTC", "contracts", "2", "1000", "0.0934", ""},
		{"missing quote", adapters.KindInversePerp, "", "contracts", "2", "100", "50000", ""},
		{"zero mark", adapters.KindLinearPerp, "USDT", "base", "2", "", "0", ""},
		{"negative mark", adapters.KindLinearPerp, "USDT", "base", "2", "", "-2", ""},
		// Binance USDⓈ-M reports base units instead of contracts.
		{"base units", adapters.KindLinearPerp, "USDT", "base", "0.5", "", "62000", "31000"},
		{"inverse quote units", adapters.KindInversePerp, "USD", "quote", "150", "100", "", "150"},
		{"incompatible linear quote units", adapters.KindLinearPerp, "USDT", "quote", "150", "", "62000", ""},
		{"incompatible inverse base units", adapters.KindInversePerp, "USD", "base", "2", "", "62000", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			account := &AccountValuation{AccountID: uuid.Must(uuid.NewV7()), Freshness: FreshnessFresh}
			row, err := positionRow(account, instrumentID[:], string(adapters.PlatformBinance), "SYMBOL", tc.kind,
				nullString("BASE"), nullString(tc.quote), nullString("BASE"), sql.NullTime{},
				nullString(tc.contractSize), "ref", adapters.SideLong, tc.quantity, tc.unit,
				sql.NullString{}, nullString(tc.mark), sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{})
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
