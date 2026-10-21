package valuation

import (
	"bytes"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// preparedReport contains only the facts needed by the evaluator. All source,
// exclusion, and price reads have completed before this value is evaluated.
type preparedReport struct {
	account    AccountValuation
	reported   *big.Rat
	balances   []preparedBalance
	adjustment *big.Rat
}

type preparedBalance struct {
	assetSymbol, canonicalAssetSymbol, rawEquity string
	equity, price                                *big.Rat
	decision                                     BalanceValuation
}

// evaluateReport is independent of storage, time, and the input loading strategy.
func evaluateReport(input preparedReport) AccountValuation {
	item := input.account
	if item.SourceReportID == nil {
		item.Method = MethodMissing
		return item
	}
	total := decimal.Add(decimal.Zero(), input.adjustment)
	item.BalanceValuations = make([]BalanceValuation, 0, len(input.balances))
	for _, balance := range input.balances {
		decision := balance.decision
		if decision.Treatment == "excluded" {
			item.BalanceValuations = append(item.BalanceValuations, decision)
			continue
		}
		if balance.equity == nil {
			decision.MissingReason = "invalid_equity"
		} else if balance.price == nil {
			decision.MissingReason = "no_price"
		} else {
			value := decimal.Mul(balance.equity, balance.price)
			decision.ValueUSD = decimal.FormatPtr(value)
			if input.reported == nil {
				total = decimal.Add(total, value)
			}
		}
		if decision.MissingReason != "" && input.reported == nil {
			item.Omitted = append(item.Omitted, Omitted{AssetID: decision.AssetID, AssetSymbol: balance.assetSymbol, CanonicalAssetSymbol: mappedCanonicalAssetSymbol(balance.assetSymbol, balance.canonicalAssetSymbol), Equity: decimal.Compact(balance.rawEquity), Reason: decision.MissingReason})
		}
		item.BalanceValuations = append(item.BalanceValuations, decision)
	}
	if input.reported != nil {
		item.Method = MethodReported
		item.valueRat = input.reported
		item.ValueUSD = decimal.FormatPtr(input.reported)
		return item
	}

	item.Method = MethodCalculated
	item.valueRat = total
	item.ValueUSD = decimal.FormatPtr(total)
	return item
}

// mappedCanonicalAssetSymbol omits the canonical asset symbol when it equals the asset symbol.
func mappedCanonicalAssetSymbol(assetSymbol, canonicalAssetSymbol string) string {
	if canonicalAssetSymbol == assetSymbol {
		return ""
	}
	return canonicalAssetSymbol
}

type previousAccount struct {
	accountType  string
	name         string
	identityID   uuid.UUID
	identityName string
	platform     string
	method       string
	complete     bool
	value        *big.Rat
}

// buildPoint aggregates account valuations and snapshots boundary events.
func buildPoint(portfolioID, cycleID uuid.UUID, evaluatedAt time.Time, accounts []AccountValuation, previous map[uuid.UUID]previousAccount) Point {
	point := Point{PortfolioID: portfolioID, CycleID: cycleID, EvaluatedAt: evaluatedAt, AccountCount: len(accounts)}
	total := decimal.Zero()
	current := map[uuid.UUID]bool{}
	for i := range accounts {
		a := &accounts[i]
		current[a.AccountID] = true
		if a.valueRat != nil {
			total = decimal.Add(total, a.valueRat)
		}
		if a.Freshness == FreshnessStale && a.Method != MethodMissing {
			point.StaleAccountCount++
		}
		if a.Completeness() == CompletenessIncomplete {
			point.IncompleteAccountCount++
		}
		if previous != nil {
			if _, existed := previous[a.AccountID]; !existed && !(a.AccountType == string(adapter.TypeManual) && identityWasPresent(previous, a.IdentityID)) {
				event := BoundaryEvent{AccountID: a.AccountID, AccountName: a.AccountName, IdentityID: a.IdentityID, IdentityName: a.IdentityName, Platform: a.Platform, Direction: DirectionContribution, Method: a.Method, ValuationComplete: a.Completeness() == CompletenessComplete, ValueUSD: a.ValueUSD, valueRat: a.valueRat}
				point.BoundaryEvents = append(point.BoundaryEvents, event)
			}
		}
	}
	if previous != nil {
		ids := make([]uuid.UUID, 0, len(previous))
		for id := range previous {
			if !current[id] {
				ids = append(ids, id)
			}
		}
		slices.SortFunc(ids, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
		for _, id := range ids {
			prev := previous[id]
			event := BoundaryEvent{AccountID: id, AccountName: prev.name, IdentityID: prev.identityID, IdentityName: prev.identityName, Platform: prev.platform, Direction: DirectionWithdrawal, Method: MethodMissing, ValuationComplete: prev.complete}
			if prev.value != nil {
				event.Method = prev.method
				event.ValueUSD = decimal.FormatPtr(prev.value)
				event.valueRat = prev.value
			}
			point.BoundaryEvents = append(point.BoundaryEvents, event)
		}
	}
	point.TotalUSD = decimal.Format(total)
	return point
}

func AccountIssues(a *AccountValuation) []Issue {
	id := a.AccountID
	var out []Issue
	switch {
	case a.Method == MethodMissing:
		detail := "No successful report exists for the current account scope; its value is unknown."
		if a.ErrorDetail != nil {
			detail += " Last error: " + *a.ErrorDetail
		}
		out = append(out, Issue{Code: "missing", AccountID: &id, AccountName: a.AccountName, Detail: detail})
	case a.Freshness == FreshnessStale:
		detail := "The account report failed this cycle; the previous successful report is carried forward."
		if a.ErrorDetail != nil {
			detail += " Last error: " + *a.ErrorDetail
		}
		out = append(out, Issue{Code: "stale", AccountID: &id, AccountName: a.AccountName, Detail: detail, ReportObservedAt: a.ReportObservedAt})
	}
	for _, o := range a.Omitted {
		reason := "its value could not be determined"
		switch o.Reason {
		case "no_price":
			reason = "no price is available"
		case "invalid_equity":
			reason = "its equity is invalid"
		}
		detail := fmt.Sprintf("Omitted from the account value because %s.", reason)
		out = append(out, Issue{Code: o.Reason, AccountID: &id, AccountName: a.AccountName, AssetID: &o.AssetID, Symbol: o.AssetSymbol, Detail: detail, OmittedBalance: &o})
	}
	return out
}
