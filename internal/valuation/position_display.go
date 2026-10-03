package valuation

import (
	"database/sql"
	"math/big"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/decimal"
)

type positionMetadata struct {
	mode, marginMode, currency, basis sql.NullString
}

// completePosition derives presentation from retained source facts. It does not
// change the original quantity, direction, or margin amount.
func completePosition(row *PositionRow, account *AccountValuation, metadata positionMetadata) {
	row.PositionMode, row.MarginMode = nullStr(metadata.mode), nullStr(metadata.marginMode)
	row.MarginCurrency, row.MarginBasis = nullStr(metadata.currency), nullStr(metadata.basis)
	row.PortfolioMargin = portfolioMargin(account)
	row.Issues = []Issue{}
	if account.Freshness == FreshnessStale {
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
	if row.Margin == nil || row.MarginCurrency == nil || row.MarginBasis == nil || row.MarginMode == nil || row.SettlementSymbol == nil {
		return nil
	}
	if *row.MarginMode == "cross" {
		if *row.MarginBasis != "initial" || row.PortfolioMargin == nil || *row.PortfolioMargin {
			return nil
		}
	} else if *row.MarginMode != "isolated" || *row.MarginBasis != "allocated" {
		return nil
	}
	amount, ok := decimal.Parse(*row.Margin)
	if !ok || amount.Sign() < 0 {
		return nil
	}
	from, to := *row.MarginCurrency, *row.SettlementSymbol
	switch {
	case from == to, usdQuote(from) && usdQuote(to):
		return decimal.FormatPtr(amount)
	case usdQuote(from) && row.BaseSymbol != nil && to == *row.BaseSymbol && row.QuoteSymbol != nil && usdQuote(*row.QuoteSymbol) && row.MarkPrice != nil:
		price, valid := decimal.Parse(*row.MarkPrice)
		if valid && price.Sign() > 0 {
			return decimal.FormatPtr(new(big.Rat).Quo(amount, price))
		}
	}
	return nil
}

func portfolioMargin(account *AccountValuation) *bool {
	var value bool
	switch adapters.AccountStructure(account.accountStructure) {
	case adapters.StructureBinancePM:
		value = true
	case adapters.StructureBinanceClassic, adapters.StructureKrakenSeparate, adapters.StructureKrakenUnified:
	case adapters.StructureOKXUnified, adapters.StructureBybitUTA2:
		if account.sourceMarginMode == nil {
			return nil
		}
		value = *account.sourceMarginMode == string(adapters.MarginOKXPM) || *account.sourceMarginMode == string(adapters.MarginBybitPM)
	default:
		return nil
	}
	return &value
}

func rowIssue(row *PositionRow, code, detail string) Issue {
	return Issue{Code: code, AccountID: &row.AccountID, AccountName: row.AccountName, Symbol: row.Symbol, Detail: detail}
}

func positionAccountIssues(account *AccountValuation) []Issue {
	out := []Issue{}
	for _, issue := range accountIssues(account) {
		// Balance-price omissions are unrelated to a Position's notional.
		if issue.Code == "missing" || issue.Code == "stale" {
			out = append(out, issue)
		}
	}
	return out
}

func positionDataIssues(issues []Issue) []Issue {
	out := []Issue{}
	for _, issue := range issues {
		if issue.Code != "stale" && issue.Code != "missing" {
			out = append(out, issue)
		}
	}
	return out
}

func positionAccount(account *AccountValuation) bool {
	switch account.AccountType {
	case "um_futures", "cm_futures", "portfolio_margin", "trading", "unified", "multi_m", "coin_m":
		return true
	case "main":
		return account.accountStructure == string(adapters.StructureKrakenUnified)
	default:
		return false
	}
}
