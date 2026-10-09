package valuation

import (
	"bytes"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// preparedReport contains only the facts needed by the evaluator. All source,
// exclusion, and price reads have completed before this value is evaluated.
type preparedReport struct {
	account  AccountValuation
	reported *big.Rat
	balances []preparedBalance
}

type preparedBalance struct {
	platformAssetSymbol, assetSymbol, rawEquity string
	equity, price                               *big.Rat
}

// evaluateReport is independent of storage, time, and the input loading strategy.
func evaluateReport(input preparedReport) AccountValuation {
	item := input.account
	if item.SourceReportID == nil {
		item.Method = MethodMissing
		return item
	}
	if input.reported != nil {
		item.Method = MethodReported
		item.valueRat = input.reported
		item.ValueUSD = decimal.FormatPtr(input.reported)
		return item
	}
	total := decimal.Zero()
	for _, balance := range input.balances {
		omission := Omitted{PlatformAssetSymbol: balance.platformAssetSymbol, AssetSymbol: mappedSymbol(balance.platformAssetSymbol, balance.assetSymbol), Equity: balance.rawEquity}
		if balance.equity == nil {
			omission.Reason = "invalid_equity"
			item.Omitted = append(item.Omitted, omission)
			continue
		}
		if balance.price == nil {
			omission.Equity = decimal.Compact(balance.rawEquity)
			omission.Reason = "no_price"
			item.Omitted = append(item.Omitted, omission)
			continue
		}
		total = decimal.Add(total, decimal.Mul(balance.equity, balance.price))
	}
	item.Method = MethodCalculated
	item.valueRat = total
	item.ValueUSD = decimal.FormatPtr(total)
	return item
}

// mappedSymbol reports the Asset symbol only when a platform asset is mapped away from its Platform symbol.
func mappedSymbol(platformAssetSymbol, assetSymbol string) string {
	if assetSymbol == platformAssetSymbol {
		return ""
	}
	return assetSymbol
}

type previousAccount struct {
	manualState  string
	scopeVersion uint16
	accountType  string
	name         string
	identityID   uuid.UUID
	identityName string
	platform     string
	method       string
	value        *big.Rat
}

// buildPoint aggregates Account Valuations into a point with issues and Boundary Events.
func buildPoint(portfolioID, jobID uuid.UUID, evaluatedAt time.Time, accounts []AccountValuation, previous map[uuid.UUID]previousAccount) Point {
	point := Point{PortfolioID: portfolioID, JobID: jobID, EvaluatedAt: evaluatedAt, AccountCount: len(accounts)}
	total := decimal.Zero()
	current := map[uuid.UUID]bool{}
	transitions := scopeTransitions(accounts, previous)
	markedTransitions := map[uuid.UUID]bool{}
	contributions := decimal.Zero()
	withdrawals := decimal.Zero()
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
		point.Issues = append(point.Issues, AccountIssues(a)...)
		if a.AccountType == string(adapters.TypeManual) && previous != nil && identityWasPresent(previous, a.IdentityID) {
			old := previous[a.AccountID].manualState
			if canonicalManualState([]byte(old)) != canonicalManualState([]byte(a.manualState)) {
				point.Issues = append(point.Issues, Issue{Code: "manual_balance_changed", AccountID: &a.AccountID, AccountName: a.AccountName, Detail: "Manual quantities changed for " + a.IdentityName + "; value changes across this point include a user adjustment."})
			}
		}
		if transitions[a.IdentityID] && (a.AccountType == "earn" || a.AccountType == "dual_investment") {
			if !markedTransitions[a.IdentityID] {
				markedTransitions[a.IdentityID] = true
				point.Issues = append(point.Issues, Issue{Code: "account_scope_changed", AccountID: &a.AccountID, Detail: "Earn and Dual Investment were separated for " + a.IdentityName + "; value changes across this point use different account scopes."})
			}
		}
		if previous != nil {
			if _, existed := previous[a.AccountID]; !existed && !(a.AccountType == string(adapters.TypeManual) && identityWasPresent(previous, a.IdentityID)) && (!transitions[a.IdentityID] || a.AccountType != "dual_investment") {
				event := BoundaryEvent{AccountID: a.AccountID, AccountName: a.AccountName, IdentityID: a.IdentityID, IdentityName: a.IdentityName, Platform: a.Platform, Direction: DirectionContribution, Method: a.Method, ValueUSD: a.ValueUSD, valueRat: a.valueRat}
				point.BoundaryEvents = append(point.BoundaryEvents, event)
				if a.valueRat != nil {
					contributions = decimal.Add(contributions, a.valueRat)
				} else {
					id := a.AccountID
					point.Issues = append(point.Issues, Issue{Code: "boundary_unvalued", AccountID: &id, AccountName: a.AccountName, Detail: "The account entered the boundary without a value; the boundary event is unvalued."})
				}
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
			if transitions[prev.identityID] && prev.accountType == "earn" {
				continue
			}
			event := BoundaryEvent{AccountID: id, AccountName: prev.name, IdentityID: prev.identityID, IdentityName: prev.identityName, Platform: prev.platform, Direction: DirectionWithdrawal, Method: MethodMissing}
			if prev.value != nil {
				event.Method = prev.method
				event.ValueUSD = decimal.FormatPtr(prev.value)
				event.valueRat = prev.value
				withdrawals = decimal.Add(withdrawals, prev.value)
			} else {
				accountID := id
				point.Issues = append(point.Issues, Issue{Code: "boundary_unvalued", AccountID: &accountID, AccountName: prev.name, Detail: "The account left the boundary without a value; the boundary event is unvalued."})
			}
			point.BoundaryEvents = append(point.BoundaryEvents, event)
		}
	}
	point.TotalUSD = decimal.Format(total)
	point.BoundaryContributionUSD = decimal.Format(contributions)
	point.BoundaryWithdrawalUSD = decimal.Format(withdrawals)
	return point
}

func AccountIssues(a *AccountValuation) []Issue {
	id := a.AccountID
	var out []Issue
	switch {
	case a.Method == MethodMissing:
		detail := "No successful report exists for the current account scope; its value is unknown."
		if a.ScopeVersion == 2 && a.AccountType == "earn" {
			detail += " Earlier combined Earn reports include Dual Investment and cannot be reused."
		}
		if a.ErrorDetail != nil {
			detail += " Last error: " + *a.ErrorDetail
		}
		out = append(out, Issue{Code: "missing", AccountID: &id, AccountName: a.AccountName, Detail: detail})
	case a.Freshness == FreshnessStale:
		detail := "The account report failed this cycle; the previous successful report is carried forward."
		if a.ErrorDetail != nil {
			detail = "The account report failed this cycle (" + *a.ErrorDetail + "); the previous successful report is carried forward."
		}
		if a.ReportObservedAt != nil {
			detail += " Observed at " + a.ReportObservedAt.UTC().Format(time.RFC3339) + "."
		}
		out = append(out, Issue{Code: "stale", AccountID: &id, AccountName: a.AccountName, Detail: detail})
	}
	for _, o := range a.Omitted {
		detail := fmt.Sprintf("%s %s was excluded from the account value (%s).", o.Equity, o.PlatformAssetSymbol, o.Reason)
		if o.AssetSymbol != "" {
			detail = fmt.Sprintf("%s %s (mapped to %s) was excluded from the account value (%s).", o.Equity, o.PlatformAssetSymbol, o.AssetSymbol, o.Reason)
		}
		out = append(out, Issue{Code: o.Reason, AccountID: &id, AccountName: a.AccountName, Symbol: o.PlatformAssetSymbol, Detail: detail})
	}
	return out
}

// scopeTransitions identifies reclassification of an already observed combined Earn.
func scopeTransitions(accounts []AccountValuation, previous map[uuid.UUID]previousAccount) map[uuid.UUID]bool {
	legacy := map[uuid.UUID]bool{}
	for _, old := range previous {
		if old.accountType == "earn" && old.scopeVersion == 1 && (old.platform == "binance" || old.platform == "okx" || old.platform == "bybit") {
			legacy[old.identityID] = true
		}
	}
	out := map[uuid.UUID]bool{}
	for i := range accounts {
		a := &accounts[i]
		if legacy[a.IdentityID] && a.ScopeVersion == 2 && (a.AccountType == "earn" || a.AccountType == "dual_investment") {
			out[a.IdentityID] = true
		}
	}
	return out
}
