package financial

import (
	"database/sql"
	"math/big"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type positionMetadata struct {
	mode, marginSetting, currency, basis sql.NullString
}

// completePosition derives presentation from retained source facts. It does not
// change the original quantity, direction, or margin amount.
func completePosition(row *PositionRow, account *valuation.AccountValuation, metadata positionMetadata) {
	completePositionFacts(row, account, metadata)
	completePositionAmounts(row)
}

func completePositionFacts(row *PositionRow, account *valuation.AccountValuation, metadata positionMetadata) {
	row.PositionMode, row.MarginSetting = nullStringPtr(metadata.mode), nullStringPtr(metadata.marginSetting)
	row.MarginAssetSymbol, row.MarginBasis = nullStringPtr(metadata.currency), nullStringPtr(metadata.basis)
	row.PortfolioMargin = portfolioMargin(account)
	if row.Platform == string(adapter.PlatformBinance) && row.AccountType == "portfolio_margin" && row.PortfolioMargin != nil && *row.PortfolioMargin && row.MarginSetting == nil {
		// The supported retail PM structure only has cross futures. This also
		// establishes the setting for old PM reports without rewriting them.
		row.MarginSetting = new("cross")
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
		row.Issues = append(row.Issues, rowIssue(row, "position_size_unknown", "Size cannot be determined without compatible quantity units, a known currency, and any required positive Contract size."))
	}
	if row.NotionalUSD == nil {
		row.Issues = append(row.Issues, rowIssue(row, "position_notional_unknown", "USD notional cannot be determined: a required currency, Contract size, or positive Mark price is unavailable."))
	}
}

func completePositionAmounts(row *PositionRow) {
	if row.Platform == string(adapter.PlatformOKX) && row.MarginSetting != nil && *row.MarginSetting == "cross" && row.PortfolioMargin != nil && !*row.PortfolioMargin {
		row.MarginAssetSymbol = verifiedOKXMarginAssetSymbol(row)
	}
	row.SettleMargin = settlePositionMargin(row)
	row.DisplayLeverage = displayPositionLeverage(row)
}

// Display leverage follows the same retained facts as the displayed margin.
func displayPositionLeverage(row *PositionRow) *string {
	if row.MarginSetting == nil {
		return nil
	}
	if *row.MarginSetting == "isolated" {
		margin, ok := positivePositionNumber(row.SettleMargin)
		var notional *big.Rat
		notionalOK := false
		if row.NotionalUSD != nil {
			notional, notionalOK = decimal.Parse(*row.NotionalUSD)
			notionalOK = notionalOK && notional.Sign() >= 0
		}
		if !ok || !notionalOK || row.SettlementAssetSymbol == nil {
			return nil
		}
		if !valuation.USDQuote(*row.SettlementAssetSymbol) {
			mark, markOK := positivePositionNumber(row.MarkPrice)
			if !markOK || row.BaseAssetSymbol == nil || *row.SettlementAssetSymbol != *row.BaseAssetSymbol || row.QuoteAssetSymbol == nil || !valuation.USDQuote(*row.QuoteAssetSymbol) {
				return nil
			}
			margin.Mul(margin, mark)
		}
		// Preserve enough division precision for the browser's final display
		// rounding, including values just below a rounding midpoint.
		value := decimal.Compact(new(big.Rat).Quo(notional, margin).FloatString(200))
		return &value
	}
	if row.PortfolioMargin != nil && *row.PortfolioMargin {
		return nil
	}
	if *row.MarginSetting == "cross" {
		if _, ok := positivePositionNumber(row.Leverage); ok {
			return row.Leverage
		}
	}
	return nil
}

func normalizedPositionSize(row *PositionRow) (*string, *string) {
	quantity, ok := decimal.Parse(row.SourceQuantity)
	if !ok {
		return nil, nil
	}
	targetUnit, symbol := adapter.UnitBase, row.BaseAssetSymbol
	if inverseKind(row.Kind) {
		targetUnit, symbol = adapter.UnitQuote, row.QuoteAssetSymbol
	}
	if symbol == nil || *symbol == "" {
		return nil, symbol
	}
	quantity = decimal.Abs(quantity)
	switch row.SourceQuantityUnit {
	case adapter.UnitContracts:
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
	if row.PositionMode != nil && *row.PositionMode == "one_way" && row.Side == adapter.SideShort {
		quantity.Neg(quantity)
	}
	return decimal.FormatPtr(quantity), symbol
}

func settlePositionMargin(row *PositionRow) *string {
	row.MarginSource, row.MarginUnavailableReason, row.MarginUnavailableCode = nil, nil, nil
	if row.MarginSetting == nil {
		return unavailablePositionMargin(row, "setting_unknown", "The position's cross or isolated setting was not recorded.")
	}
	if *row.MarginSetting == "cross" {
		if row.PortfolioMargin == nil {
			return unavailablePositionMargin(row, "account_mode_unknown", "The source report's account margin mode is unknown, so an individual initial requirement cannot be verified.")
		}
		if *row.PortfolioMargin && !binanceCrossMargin(row) {
			return unavailablePositionMargin(row, "shared_portfolio_margin", "This portfolio margin regime does not provide an independent position initial requirement. Portfolio risk is not allocated to individual positions.")
		}
	} else if *row.MarginSetting != "isolated" {
		return unavailablePositionMargin(row, "setting_unsupported", "The position margin setting is unsupported.")
	} else if row.MarginBasis == nil || *row.MarginBasis != "allocated" {
		return unavailablePositionMargin(row, "allocated_margin_unavailable", "Allocated margin excluding PnL was not provided. An initial requirement cannot be used as the allocated amount.")
	}
	if row.Margin != nil && row.MarginAssetSymbol != nil && row.MarginBasis != nil && (*row.MarginSetting != "cross" || *row.MarginBasis == "initial") {
		amount, ok := decimal.Parse(*row.Margin)
		if !ok || amount.Sign() < 0 {
			return unavailablePositionMargin(row, "reported_margin_invalid", "The reported position margin amount is invalid.")
		}
		if value := convertPositionMargin(row, amount, *row.MarginAssetSymbol); value != nil {
			row.MarginSource = new("reported")
			return value
		}
	}
	if binanceCrossMargin(row) {
		return calculatedBinanceMargin(row)
	}
	if row.Margin == nil {
		return unavailablePositionMargin(row, "reported_margin_unavailable", "The platform did not provide this position margin amount.")
	}
	if row.MarginBasis == nil || *row.MarginSetting == "cross" && *row.MarginBasis != "initial" {
		return unavailablePositionMargin(row, "initial_margin_unavailable", "A position initial requirement excluding orders was not recorded.")
	}
	if row.MarginAssetSymbol == nil || row.SettlementAssetSymbol == nil {
		return unavailablePositionMargin(row, "currency_unknown", "The source margin currency or settle currency cannot be verified.")
	}
	return unavailablePositionMargin(row, "conversion_unavailable", "The reported amount cannot be converted to Settle: compatible currencies and any required positive Mark price are unavailable.")
}

func convertPositionMargin(row *PositionRow, amount *big.Rat, from string) *string {
	if row.SettlementAssetSymbol == nil {
		return nil
	}
	to := *row.SettlementAssetSymbol
	switch {
	case from == to, valuation.USDQuote(from) && valuation.USDQuote(to):
		return decimal.FormatPtr(amount)
	case valuation.USDQuote(from) && row.BaseAssetSymbol != nil && to == *row.BaseAssetSymbol && row.QuoteAssetSymbol != nil && valuation.USDQuote(*row.QuoteAssetSymbol) && row.MarkPrice != nil:
		price, valid := decimal.Parse(*row.MarkPrice)
		if valid && price.Sign() > 0 {
			return decimal.FormatPtr(new(big.Rat).Quo(amount, price))
		}
	}
	return nil
}

func binanceCrossMargin(row *PositionRow) bool {
	if row.Platform != string(adapter.PlatformBinance) || row.MarginSetting == nil || *row.MarginSetting != "cross" || row.PortfolioMargin == nil {
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
	case adapter.KindLinearPerpetual, adapter.KindLinearDelivery, adapter.KindInversePerpetual, adapter.KindInverseDelivery:
	default:
		return unavailablePositionMargin(row, "contract_kind_unsupported", "Initial margin calculation requires a supported contract kind.")
	}
	size, _ := normalizedPositionSize(row)
	if size == nil || row.QuoteAssetSymbol == nil || !valuation.USDQuote(*row.QuoteAssetSymbol) {
		return unavailablePositionMargin(row, "size_or_quote_unavailable", "Initial margin calculation requires a known Size and USD-equivalent Quote.")
	}
	mark, markOK := positivePositionNumber(row.MarkPrice)
	leverage, leverageOK := positivePositionNumber(row.Leverage)
	if !markOK || !leverageOK {
		return unavailablePositionMargin(row, "mark_or_leverage_unavailable", "Initial margin calculation requires a positive Mark price and the position's positive initial Leverage.")
	}
	notional := decimal.Abs(decimal.ParseOrZero(*size))
	if inverseKind(row.Kind) {
		if row.BaseAssetSymbol == nil || row.SettlementAssetSymbol == nil || *row.BaseAssetSymbol != *row.SettlementAssetSymbol {
			return unavailablePositionMargin(row, "settlement_mismatch", "Inverse initial margin calculation requires base to be the settle currency.")
		}
	} else {
		notional.Mul(notional, mark)
	}
	initial := new(big.Rat).Quo(notional, leverage)
	value := convertPositionMargin(row, initial, *row.QuoteAssetSymbol)
	if value == nil {
		return unavailablePositionMargin(row, "conversion_unavailable", "Calculated initial margin cannot be converted to settle without compatible contract currencies.")
	}
	row.MarginSource = new("calculated")
	return value
}

// OKX documents position imr as USD, but reviewed non-PM inverse responses
// match native settlement IM exactly. Verify both documented candidates rather
// than dividing a native amount by mark again. Applies to old reports too;
// stored amounts, units and report modes are not rewritten.
func verifiedOKXMarginAssetSymbol(row *PositionRow) *string {
	if row.MarginAssetSymbol == nil || row.MarginBasis == nil || *row.MarginBasis != "initial" || row.SettlementAssetSymbol == nil {
		return nil
	}
	from, settle := *row.MarginAssetSymbol, *row.SettlementAssetSymbol
	if valuation.USDQuote(from) && valuation.USDQuote(settle) {
		return row.MarginAssetSymbol
	}
	if !inverseKind(row.Kind) || row.BaseAssetSymbol == nil || *row.BaseAssetSymbol != settle || row.QuoteAssetSymbol == nil || !valuation.USDQuote(*row.QuoteAssetSymbol) || from != settle && !valuation.USDQuote(from) {
		return nil
	}
	actual, actualOK := positivePositionNumber(row.Margin)
	if row.Margin != nil && decimal.ParseOrZero(*row.Margin).Sign() == 0 && decimal.Valid(*row.Margin) {
		return row.SettlementAssetSymbol
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
		return row.SettlementAssetSymbol
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

func unavailablePositionMargin(row *PositionRow, code, reason string) *string {
	row.MarginUnavailableReason = new(reason)
	row.MarginUnavailableCode = new(code)
	return nil
}

func portfolioMargin(account *valuation.AccountValuation) *bool {
	var value bool
	switch adapter.AccountStructure(account.AccountStructure) {
	case adapter.StructureBinancePM:
		value = true
	case adapter.StructureBinanceClassic, adapter.StructureKrakenSeparate, adapter.StructureKrakenUnified:
	case adapter.StructureOKXUnified, adapter.StructureBybitUTA2:
		if account.SourceMarginMode == nil {
			return nil
		}
		value = *account.SourceMarginMode == string(adapter.MarginOKXPM) || *account.SourceMarginMode == string(adapter.MarginBybitPM)
	default:
		return nil
	}
	return &value
}

func rowIssue(row *PositionRow, code, detail string) valuation.Issue {
	return valuation.Issue{Code: code, AccountID: &row.AccountID, AccountName: row.AccountName, Symbol: row.FuturesSymbol, Detail: detail}
}

func positionAccountIssues(account *valuation.AccountValuation) []valuation.Issue {
	out := []valuation.Issue{}
	for _, issue := range valuation.AccountIssues(account) {
		// Balance-price omissions are unrelated to a position's notional.
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
		return account.AccountStructure == string(adapter.StructureKrakenUnified)
	default:
		return false
	}
}
