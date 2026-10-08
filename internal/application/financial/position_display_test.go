package financial

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestPositionSizeDirectionAndUnknowns(t *testing.T) {
	for _, tc := range []struct{ name, kind, unit, quantity, factor, mode, side, want, currency string }{
		{"linear perpetual contracts", adapters.KindLinearPerpetual, "contracts", "3", "0.01", "one_way", "short", "-0.03", "BTC"},
		{"one-way long", adapters.KindLinearPerpetual, "base", "0.2", "100", "one_way", "long", "0.2", "BTC"},
		{"linear delivery base not multiplied", adapters.KindLinearDelivery, "base", "2", "100", "hedged", "short", "2", "BTC"},
		{"inverse perpetual contracts", adapters.KindInversePerpetual, "contracts", "4", "100", "one_way", "short", "-400", "USD"},
		{"inverse delivery quote not multiplied", adapters.KindInverseDelivery, "quote", "500", "100", "hedged", "long", "500", "USD"},
		{"unknown mode preserves direction and absolute size", adapters.KindLinearPerpetual, "base", "2", "", "", "short", "2", "BTC"},
		{"missing factor", adapters.KindInversePerpetual, "contracts", "4", "", "one_way", "long", "", "USD"},
		{"zero factor", adapters.KindLinearPerpetual, "contracts", "4", "0", "hedged", "long", "", "BTC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &valuation.AccountValuation{AccountID: uuid.New(), Freshness: valuation.FreshnessFresh, AccountStructure: string(adapters.StructureBinanceClassic)}
			id := uuid.New()
			row, err := positionRow(account, &sqlcgen.ListPositionsByReportRow{
				InstrumentID:        id[:],
				Platform:            "binance",
				Symbol:              "BTCUSD",
				Kind:                tc.kind,
				BaseSymbol:          nullString("BTC"),
				QuoteSymbol:         nullString("USD"),
				SettlementSymbol:    nullString("BTC"),
				ContractSize:        nullString(tc.factor),
				PlatformPositionRef: "ref",
				Side:                tc.side,
				Quantity:            tc.quantity,
				QuantityUnit:        tc.unit,
			})
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
		{"native cross", "cross", "initial", "USDT", "USDT", "", new(false), "100"},
		{"stablecoin conversion", "cross", "initial", "USD", "USDC", "", new(false), "100"},
		{"inverse conversion", "cross", "initial", "USD", "BTC", "50000", new(false), "0.002"},
		{"missing price", "cross", "initial", "USD", "BTC", "", new(false), ""},
		{"zero price", "cross", "initial", "USD", "BTC", "0", new(false), ""},
		{"negative price", "cross", "initial", "USD", "BTC", "-2", new(false), ""},
		{"allocation", "isolated", "allocated", "BTC", "BTC", "", new(false), "100"},
		{"legacy isolated initial is not allocation", "isolated", "initial", "BTC", "BTC", "", new(false), ""},
		{"portfolio cross risk is not allocation", "cross", "initial", "USD", "USD", "", new(true), ""},
		{"portfolio isolated allocation", "isolated", "allocated", "USD", "USD", "", new(true), "100"},
		{"unknown account regime", "cross", "initial", "USD", "USD", "", nil, ""},
		{"unrelated currency", "cross", "initial", "EUR", "BTC", "50000", new(false), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := PositionRow{Margin: new("100"), MarginMode: new(tc.mode), MarginBasis: new(tc.basis), MarginCurrency: new(tc.from), SettlementSymbol: new(tc.to), BaseSymbol: new("BTC"), QuoteSymbol: new("USD"), MarkPrice: nullDecimal(nullString(tc.mark)), PortfolioMargin: tc.portfolio}
			if got := stringValue(settlePositionMargin(&row)); got != tc.want {
				t.Fatalf("settle Margin=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestPortfolioUsesSourceReportMode(t *testing.T) {
	account := &valuation.AccountValuation{AccountStructure: string(adapters.StructureBybitUTA2), SourceMarginMode: new(string(adapters.MarginBybitPM))}
	if value := portfolioMargin(account); value == nil || !*value {
		t.Fatal("source Portfolio mode lost")
	}
	account.SourceMarginMode = nil
	if portfolioMargin(account) != nil {
		t.Fatal("unknown historical mode was guessed")
	}
}

func TestBinanceCrossMarginCalculationAndSourcePreference(t *testing.T) {
	for _, tc := range []struct{ name, kind, unit, quantity, factor, settle, want string }{
		{"linear perpetual Base", adapters.KindLinearPerpetual, "base", "-0.5", "100", "USDT", "250"},
		{"linear delivery contracts", adapters.KindLinearDelivery, "contracts", "5", "0.1", "USDT", "250"},
		{"inverse perpetual contracts", adapters.KindInversePerpetual, "contracts", "5", "100", "BTC", "0.25"},
		{"inverse delivery Quote", adapters.KindInverseDelivery, "quote", "500", "100", "BTC", "0.25"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := PositionRow{
				Platform: "binance", AccountType: "portfolio_margin", Kind: tc.kind,
				Quantity: tc.quantity, QuantityUnit: tc.unit, ContractSize: new(tc.factor),
				BaseSymbol: new("BTC"), QuoteSymbol: new("USDT"), SettlementSymbol: new(tc.settle),
				MarkPrice: new("1000"), Leverage: new("2"), MarginMode: new("cross"), PortfolioMargin: new(true),
			}
			if got := stringValue(settlePositionMargin(&row)); got != tc.want || stringValue(row.MarginSource) != "calculated" || row.MarginUnavailableReason != nil {
				t.Fatalf("margin=%q source=%v reason=%v", got, row.MarginSource, row.MarginUnavailableReason)
			}
			if row.Margin != nil || row.MarginBasis != nil || row.Quantity != tc.quantity {
				t.Fatal("calculation changed a source amount or basis")
			}
			row.Margin, row.MarginCurrency, row.MarginBasis = new("7"), new(tc.settle), new("initial")
			if got := stringValue(settlePositionMargin(&row)); got != "7" || stringValue(row.MarginSource) != "reported" {
				t.Fatalf("reliable source was not preferred: %q %+v", got, row)
			}
		})
	}
	// Binance screenshots approximately match this calculation; display prices
	// and exchange UI updates are not an atomic reconciliation.
	row := PositionRow{
		Platform: "binance", AccountType: "portfolio_margin", Kind: adapters.KindLinearPerpetual,
		Quantity: "0.5", QuantityUnit: "base", BaseSymbol: new("ZEC"), QuoteSymbol: new("USDT"), SettlementSymbol: new("USDT"),
		MarkPrice: new("1317.81"), Leverage: new("3"), MarginMode: new("cross"), PortfolioMargin: new(true),
	}
	if got := stringValue(settlePositionMargin(&row)); got != "219.635" {
		t.Fatalf("Binance example %q", got)
	}
	// A missing report amount in Classic Cross may use the same formula.
	row.AccountType, row.PortfolioMargin = "um_futures", new(false)
	if stringValue(settlePositionMargin(&row)) != "219.635" {
		t.Fatal("Classic Cross fallback unavailable")
	}
}

func TestCalculatedMarginUnknownsAndPlatformLimits(t *testing.T) {
	base := PositionRow{
		Platform: "binance", AccountType: "portfolio_margin", Kind: adapters.KindInverseDelivery,
		Quantity: "2", QuantityUnit: "contracts", ContractSize: new("100"),
		BaseSymbol: new("BTC"), QuoteSymbol: new("USD"), SettlementSymbol: new("BTC"),
		MarkPrice: new("50000"), Leverage: new("10"), MarginMode: new("cross"), PortfolioMargin: new(true),
	}
	for _, tc := range []struct {
		name   string
		change func(*PositionRow)
	}{
		{"missing factor", func(r *PositionRow) { r.ContractSize = nil }},
		{"zero leverage", func(r *PositionRow) { r.Leverage = new("0") }},
		{"missing leverage", func(r *PositionRow) { r.Leverage = nil }},
		{"missing Mark", func(r *PositionRow) { r.MarkPrice = nil }},
		{"negative Mark", func(r *PositionRow) { r.MarkPrice = new("-1") }},
		{"unknown currency", func(r *PositionRow) { r.QuoteSymbol = nil }},
		{"incompatible inverse Settle", func(r *PositionRow) { r.SettlementSymbol = new("USDT") }},
		{"Isolated is never calculated", func(r *PositionRow) { r.MarginMode = new("isolated"); r.MarginBasis = new("initial") }},
		{"Bybit PM risk not allocated", func(r *PositionRow) { r.Platform = "bybit"; r.AccountType = "unified" }},
		{"OKX PM risk not allocated", func(r *PositionRow) { r.Platform = "okx"; r.AccountType = "trading" }},
		{"Kraken effective leverage not inverted", func(r *PositionRow) {
			r.Platform = "kraken"
			r.AccountType = "coin_m"
			r.PortfolioMargin = new(false)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			tc.change(&row)
			if settlePositionMargin(&row) != nil || row.MarginSource != nil || row.MarginUnavailableReason == nil {
				t.Fatalf("unknown margin or reason mishandled: %+v", row)
			}
		})
	}
	// Bybit's source includes fee/hedge treatment; preserve it rather than
	// replacing it with the Binance formula, even with the same inputs.
	base.Platform, base.AccountType, base.PortfolioMargin = "bybit", "unified", new(false)
	base.Margin, base.MarginCurrency, base.MarginBasis = new("0.00966596"), new("BTC"), new("initial")
	if stringValue(settlePositionMargin(&base)) != "0.00966596" || stringValue(base.MarginSource) != "reported" {
		t.Fatal("Bybit reported requirement was replaced")
	}
}

func TestLegacyBinancePMUsesStoredPositionFacts(t *testing.T) {
	account := &valuation.AccountValuation{AccountStructure: string(adapters.StructureBinancePM), Freshness: valuation.FreshnessFresh}
	row := PositionRow{
		Platform: "binance", AccountType: "portfolio_margin", Kind: adapters.KindLinearPerpetual,
		Quantity: "0.5", QuantityUnit: "base", BaseSymbol: new("BTC"), QuoteSymbol: new("USDT"), SettlementSymbol: new("USDT"),
		MarkPrice: new("1000"), Leverage: new("2"), NotionalUSD: new("500"),
	}
	completePosition(&row, account, positionMetadata{mode: nullString("one_way")})
	if stringValue(row.SettleMargin) != "250" || stringValue(row.MarginMode) != "cross" || stringValue(row.MarginSource) != "calculated" || len(row.Issues) != 0 {
		t.Fatalf("old PM report %+v", row)
	}
}

func TestOKXInverseMarginUnitsAreVerifiedOnce(t *testing.T) {
	base := PositionRow{
		Platform: "okx", Kind: adapters.KindInversePerpetual, Quantity: "2", QuantityUnit: "contracts", ContractSize: new("10"),
		BaseSymbol: new("ETH"), QuoteSymbol: new("USD"), SettlementSymbol: new("ETH"), MarkPrice: new("2687.06"), Leverage: new("3"),
		Margin: new("0.0024810263509809"), MarginCurrency: new("USD"), MarginBasis: new("initial"), MarginMode: new("cross"), PortfolioMargin: new(false),
	}
	for _, from := range []string{"USD", "ETH"} {
		row := base
		row.MarginCurrency = new(from)
		row.MarginCurrency = verifiedOKXMarginCurrency(&row)
		if stringValue(row.MarginCurrency) != "ETH" || stringValue(settlePositionMargin(&row)) != "0.0024810263509809" {
			t.Fatalf("native amount converted twice: %+v", row)
		}
	}
	usd := base
	usd.Margin = new("6.666666666666667")
	if stringValue(verifiedOKXMarginCurrency(&usd)) != "USD" {
		t.Fatal("documented USD candidate was not recognized")
	}
	for _, tc := range []struct {
		name   string
		change func(*PositionRow)
	}{
		{"neither candidate", func(r *PositionRow) { r.Margin = new("17") }},
		{"ambiguous candidates", func(r *PositionRow) { r.MarkPrice = new("1"); r.Margin = new("6.666666666666667") }},
		{"missing source currency", func(r *PositionRow) { r.MarginCurrency = nil }},
		{"unrelated currency", func(r *PositionRow) { r.MarginCurrency = new("EUR") }},
		{"missing contract factor", func(r *PositionRow) { r.ContractSize = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			tc.change(&row)
			if verifiedOKXMarginCurrency(&row) != nil {
				t.Fatal("unverified source unit guessed")
			}
		})
	}
	if !marginMatches(decimal.ParseOrZero("100.000001"), decimal.ParseOrZero("100")) || marginMatches(decimal.ParseOrZero("100.0000011"), decimal.ParseOrZero("100")) {
		t.Fatal("unit-validation tolerance boundary changed")
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
