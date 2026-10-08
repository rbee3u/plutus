package financial

import (
	"database/sql"
	"math/big"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type positionMetadata struct {
	mode, marginMode, currency, basis sql.NullString
}

// completePosition derives presentation from retained source facts. It does not
// change the original quantity, direction, or margin amount.
func completePosition(row *PositionRow, account *valuation.AccountValuation, metadata positionMetadata) {
	row.PositionMode, row.MarginMode = nullStringPtr(metadata.mode), nullStringPtr(metadata.marginMode)
	row.MarginCurrency, row.MarginBasis = nullStringPtr(metadata.currency), nullStringPtr(metadata.basis)
	row.PortfolioMargin = portfolioMargin(account)
	if row.Platform == string(adapters.PlatformBinance) && row.AccountType == "portfolio_margin" && row.PortfolioMargin != nil && *row.PortfolioMargin && row.MarginMode == nil {
		// The supported retail PM structure only has Cross futures. This also
		// establishes the setting for old PM Reports without rewriting them.
		row.MarginMode = new("cross")
	}
	row.Issues = []valuation.Issue{}
	if account.Freshness == valuation.FreshnessStale {
		row.Issues = append(row.Issues, positionAccountIssues(account)...)
	}
	if row.PositionMode == nil {
		row.Issues = append(row.Issues, rowIssue(row, "position_mode_unknown", "Position mode was not recorded; the known direction is shown with a positive Size."))
	}
	row.Size, row.SizeSymbol = normalizedPositionSize(row)
	if row.Size == nil {
		row.Issues = append(row.Issues, rowIssue(row, "position_size_unknown", "Size cannot be determined without compatible quantity units, a known currency, and any required positive Contract Size."))
	}
	if row.NotionalUSD == nil {
		row.Issues = append(row.Issues, rowIssue(row, "position_notional_unknown", "USD notional cannot be determined: a required currency, Contract Size, or positive Mark Price is unavailable."))
	}
	if row.Platform == string(adapters.PlatformOKX) && row.MarginMode != nil && *row.MarginMode == "cross" && row.PortfolioMargin != nil && !*row.PortfolioMargin {
		row.MarginCurrency = verifiedOKXMarginCurrency(row)
	}
	row.SettleMargin = settlePositionMargin(row)
}

func normalizedPositionSize(row *PositionRow) (*string, *string) {
	quantity, ok := decimal.Parse(row.Quantity)
	if !ok {
		return nil, nil
	}
	targetUnit, symbol := adapters.UnitBase, row.BaseSymbol
	if inverseKind(row.Kind) {
		targetUnit, symbol = adapters.UnitQuote, row.QuoteSymbol
	}
	if symbol == nil || *symbol == "" {
		return nil, symbol
	}
	quantity = decimal.Abs(quantity)
	switch row.QuantityUnit {
	case adapters.UnitContracts:
		if row.ContractSize == nil {
			return nil, symbol
		}
		factor, valid := decimal.Parse(*row.ContractSize)
		if !valid || factor.Sign() <= 0 {
			return nil, symbol
		}
		quantity = decimal.Mul(quantity, factor)
	case targetUnit:
	default:
		return nil, symbol
	}
	if row.PositionMode != nil && *row.PositionMode == "one_way" && row.Side == adapters.SideShort {
		quantity.Neg(quantity)
	}
	return decimal.FormatPtr(quantity), symbol
}

func settlePositionMargin(row *PositionRow) *string {
	row.MarginSource, row.MarginUnavailableReason = nil, nil
	if row.MarginMode == nil {
		return unavailablePositionMargin(row, "The position's Cross or Isolated setting was not recorded.")
	}
	if *row.MarginMode == "cross" {
		if row.PortfolioMargin == nil {
			return unavailablePositionMargin(row, "The source Report's Account Margin Mode is unknown, so an individual initial requirement cannot be verified.")
		}
		if *row.PortfolioMargin && !binanceCrossMargin(row) {
			return unavailablePositionMargin(row, "This Portfolio Margin regime does not provide an independent position initial requirement. Portfolio risk is not allocated to individual positions.")
		}
	} else if *row.MarginMode != "isolated" {
		return unavailablePositionMargin(row, "The position margin setting is unsupported.")
	} else if row.MarginBasis == nil || *row.MarginBasis != "allocated" {
		return unavailablePositionMargin(row, "Allocated margin excluding PnL was not provided. An initial requirement cannot be used as the allocated amount.")
	}
	if row.Margin != nil && row.MarginCurrency != nil && row.MarginBasis != nil && (*row.MarginMode != "cross" || *row.MarginBasis == "initial") {
		amount, ok := decimal.Parse(*row.Margin)
		if !ok || amount.Sign() < 0 {
			return unavailablePositionMargin(row, "The reported position margin amount is invalid.")
		}
		if value := convertPositionMargin(row, amount, *row.MarginCurrency); value != nil {
			row.MarginSource = new("reported")
			return value
		}
	}
	if binanceCrossMargin(row) {
		return calculatedBinanceMargin(row)
	}
	if row.Margin == nil {
		return unavailablePositionMargin(row, "The Platform did not provide this position margin amount.")
	}
	if row.MarginBasis == nil || *row.MarginMode == "cross" && *row.MarginBasis != "initial" {
		return unavailablePositionMargin(row, "A position initial requirement excluding orders was not recorded.")
	}
	if row.MarginCurrency == nil || row.SettlementSymbol == nil {
		return unavailablePositionMargin(row, "The source margin currency or Settle currency cannot be verified.")
	}
	return unavailablePositionMargin(row, "The reported amount cannot be converted to Settle: compatible currencies and any required positive Mark Price are unavailable.")
}

func convertPositionMargin(row *PositionRow, amount *big.Rat, from string) *string {
	if row.SettlementSymbol == nil {
		return nil
	}
	to := *row.SettlementSymbol
	switch {
	case from == to, valuation.USDQuote(from) && valuation.USDQuote(to):
		return decimal.FormatPtr(amount)
	case valuation.USDQuote(from) && row.BaseSymbol != nil && to == *row.BaseSymbol && row.QuoteSymbol != nil && valuation.USDQuote(*row.QuoteSymbol) && row.MarkPrice != nil:
		price, valid := decimal.Parse(*row.MarkPrice)
		if valid && price.Sign() > 0 {
			return decimal.FormatPtr(new(big.Rat).Quo(amount, price))
		}
	}
	return nil
}

func binanceCrossMargin(row *PositionRow) bool {
	if row.Platform != string(adapters.PlatformBinance) || row.MarginMode == nil || *row.MarginMode != "cross" || row.PortfolioMargin == nil {
		return false
	}
	switch row.AccountType {
	case "um_futures", "cm_futures", "portfolio_margin":
		return true
	default:
		return false
	}
}

// calculatedBinanceMargin follows Binance's documented initial requirement,
// including retail PM. It never distributes shared collateral or order margin.
func calculatedBinanceMargin(row *PositionRow) *string {
	switch row.Kind {
	case adapters.KindLinearPerpetual, adapters.KindLinearDelivery, adapters.KindInversePerpetual, adapters.KindInverseDelivery:
	default:
		return unavailablePositionMargin(row, "Initial margin calculation requires a supported contract kind.")
	}
	size, _ := normalizedPositionSize(row)
	if size == nil || row.QuoteSymbol == nil || !valuation.USDQuote(*row.QuoteSymbol) {
		return unavailablePositionMargin(row, "Initial margin calculation requires a known Size and USD-equivalent Quote.")
	}
	mark, markOK := positivePositionNumber(row.MarkPrice)
	leverage, leverageOK := positivePositionNumber(row.Leverage)
	if !markOK || !leverageOK {
		return unavailablePositionMargin(row, "Initial margin calculation requires a positive Mark Price and the position's positive initial Leverage.")
	}
	notional := decimal.Abs(decimal.ParseOrZero(*size))
	if inverseKind(row.Kind) {
		if row.BaseSymbol == nil || row.SettlementSymbol == nil || *row.BaseSymbol != *row.SettlementSymbol {
			return unavailablePositionMargin(row, "Inverse initial margin calculation requires Base to be the Settle currency.")
		}
	} else {
		notional.Mul(notional, mark)
	}
	initial := new(big.Rat).Quo(notional, leverage)
	value := convertPositionMargin(row, initial, *row.QuoteSymbol)
	if value == nil {
		return unavailablePositionMargin(row, "Calculated initial margin cannot be converted to Settle without compatible contract currencies.")
	}
	row.MarginSource = new("calculated")
	return value
}

// OKX documents position imr as USD, but reviewed non-PM inverse responses
// match native settlement IM exactly. Verify both documented candidates rather
// than dividing a native amount by Mark again. Applies to old Reports too;
// stored amounts, units and Report modes are not rewritten.
func verifiedOKXMarginCurrency(row *PositionRow) *string {
	if row.MarginCurrency == nil || row.MarginBasis == nil || *row.MarginBasis != "initial" || row.SettlementSymbol == nil {
		return nil
	}
	from, settle := *row.MarginCurrency, *row.SettlementSymbol
	if valuation.USDQuote(from) && valuation.USDQuote(settle) {
		return row.MarginCurrency
	}
	if !inverseKind(row.Kind) || row.BaseSymbol == nil || *row.BaseSymbol != settle || row.QuoteSymbol == nil || !valuation.USDQuote(*row.QuoteSymbol) || from != settle && !valuation.USDQuote(from) {
		return nil
	}
	actual, actualOK := positivePositionNumber(row.Margin)
	if row.Margin != nil && decimal.ParseOrZero(*row.Margin).Sign() == 0 && decimal.Valid(*row.Margin) {
		return row.SettlementSymbol
	}
	size, _ := normalizedPositionSize(row)
	mark, markOK := positivePositionNumber(row.MarkPrice)
	leverage, leverageOK := positivePositionNumber(row.Leverage)
	if !actualOK || size == nil || !markOK || !leverageOK {
		return nil
	}
	usd := new(big.Rat).Quo(decimal.Abs(decimal.ParseOrZero(*size)), leverage)
	native := new(big.Rat).Quo(usd, mark)
	nativeMatch, usdMatch := marginMatches(actual, native), marginMatches(actual, usd)
	if nativeMatch && !usdMatch {
		return row.SettlementSymbol
	}
	if usdMatch && !nativeMatch {
		return new("USD")
	}
	return nil
}

func marginMatches(actual, expected *big.Rat) bool {
	// The accepted observations differ by around 1e-14 relative rounding. The
	// 1e-8 bound covers decimal rounding without using amount magnitude as a unit.
	tolerance := new(big.Rat).Quo(decimal.Abs(expected), big.NewRat(100000000, 1))
	return decimal.Abs(decimal.Sub(actual, expected)).Cmp(tolerance) <= 0
}

func positivePositionNumber(value *string) (*big.Rat, bool) {
	if value == nil {
		return nil, false
	}
	amount, valid := decimal.Parse(*value)
	return amount, valid && amount.Sign() > 0
}

func unavailablePositionMargin(row *PositionRow, reason string) *string {
	row.MarginUnavailableReason = new(reason)
	return nil
}

func portfolioMargin(account *valuation.AccountValuation) *bool {
	var value bool
	switch adapters.AccountStructure(account.AccountStructure) {
	case adapters.StructureBinancePM:
		value = true
	case adapters.StructureBinanceClassic, adapters.StructureKrakenSeparate, adapters.StructureKrakenUnified:
	case adapters.StructureOKXUnified, adapters.StructureBybitUTA2:
		if account.SourceMarginMode == nil {
			return nil
		}
		value = *account.SourceMarginMode == string(adapters.MarginOKXPM) || *account.SourceMarginMode == string(adapters.MarginBybitPM)
	default:
		return nil
	}
	return &value
}

func rowIssue(row *PositionRow, code, detail string) valuation.Issue {
	return valuation.Issue{Code: code, AccountID: &row.AccountID, AccountName: row.AccountName, Symbol: row.Symbol, Detail: detail}
}

func positionAccountIssues(account *valuation.AccountValuation) []valuation.Issue {
	out := []valuation.Issue{}
	for _, issue := range valuation.AccountIssues(account) {
		// Balance-price omissions are unrelated to a Position's notional.
		if issue.Code == "missing" || issue.Code == "stale" {
			out = append(out, issue)
		}
	}
	return out
}

func positionDataIssues(issues []valuation.Issue) []valuation.Issue {
	out := []valuation.Issue{}
	for _, issue := range issues {
		if issue.Code != "stale" && issue.Code != "missing" {
			out = append(out, issue)
		}
	}
	return out
}

func positionAccount(account *valuation.AccountValuation) bool {
	switch account.AccountType {
	case "um_futures", "cm_futures", "portfolio_margin", "trading", "unified", "multi_m", "coin_m":
		return true
	case "main":
		return account.AccountStructure == string(adapters.StructureKrakenUnified)
	default:
		return false
	}
}
