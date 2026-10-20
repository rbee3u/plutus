package financial

import (
	"context"
	"errors"
	"math/big"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type holdingTotal struct {
	value        *big.Rat
	known        bool
	completeness string
	freshness    string
	issues       []valuation.Issue
}

func newHoldingTotal(accounts []valuation.AccountValuation, f *ListFilter, kind string, hasPoint bool) *holdingTotal {
	t := &holdingTotal{value: decimal.Zero(), completeness: valuation.CompletenessComplete, freshness: valuation.FreshnessFresh, issues: []valuation.Issue{}}
	if !hasPoint {
		t.completeness = valuation.CompletenessIncomplete
		t.issues = append(t.issues, valuation.Issue{Code: "no_point", Detail: "No collection cycle has produced a valuation point yet."})
	}
	for i := range accounts {
		a := &accounts[i]
		if !matchesHoldingAccount(f, a, kind) {
			continue
		}
		if a.SourceReportID == nil {
			t.completeness = valuation.CompletenessIncomplete
			t.issues = append(t.issues, valuation.Issue{Code: "missing", AccountID: &a.AccountID, AccountName: a.AccountName, Detail: "No compatible successful account report supplies holdings for the filtered total."})
		} else if a.Freshness == valuation.FreshnessStale {
			t.freshness = valuation.FreshnessStale
			t.issues = append(t.issues, staleAccountIssue(a))
		}
	}
	return t
}

func (t *holdingTotal) add(value *big.Rat) {
	if value != nil {
		t.value = decimal.Add(t.value, value)
		t.known = true
	}
}

func (l *FlatList[T]) setTotal(t *holdingTotal) {
	l.TotalUSD = nil
	if t.known || t.completeness == valuation.CompletenessComplete {
		l.TotalUSD = decimal.FormatPtr(t.value)
	}
	l.TotalCompleteness, l.TotalFreshness, l.TotalIssues = t.completeness, t.freshness, t.issues
}

type productHolding struct {
	accountID uuid.UUID
	symbol    string
	amount    string
}

// Product totals reuse adopted balance prices, including historical stale prices.
// Group quantities before pricing so omitted assets are named once per account.
func (s *Service) totalProductHoldings(ctx context.Context, p PortfolioContext, accounts []valuation.AccountValuation, holdings []productHolding, total *holdingTotal) error {
	type key struct {
		accountID uuid.UUID
		symbol    string
	}
	amounts := map[key]*big.Rat{}
	for _, h := range holdings {
		amount, ok := decimal.Parse(h.amount)
		if !ok {
			return errors.New("invalid product holding amount")
		}
		k := key{h.accountID, h.symbol}
		if amounts[k] == nil {
			amounts[k] = decimal.Zero()
		}
		amounts[k] = decimal.Add(amounts[k], amount)
	}
	if len(amounts) == 0 {
		return nil
	}
	book := valuation.NewPriceBook(mysql.Queries(ctx, s.db), p.ID)
	if err := book.PreloadObservations(ctx, accounts); err != nil {
		return err
	}
	ids := [][]byte{}
	byReport := map[uuid.UUID]*valuation.AccountValuation{}
	decisions := map[uuid.UUID]map[uuid.UUID]*valuation.BalanceValuation{}
	for i := range accounts {
		a := &accounts[i]
		if a.SourceReportID != nil {
			ids = append(ids, a.SourceReportID[:])
			byReport[*a.SourceReportID] = a
			decisions[*a.SourceReportID] = balanceDecisions(a)
		}
	}
	for start := 0; start < len(ids); start += 500 {
		rows, err := mysql.Queries(ctx, s.db).ListBalancesByReports(ctx, sqlcgen.ListBalancesByReportsParams{PortfolioID: p.ID[:], ReportIds: ids[start:min(start+500, len(ids))]})
		if err != nil {
			return err
		}
		for _, raw := range rows {
			reportID, err := uuid.FromBytes(raw.ReportID)
			if err != nil {
				return err
			}
			account := byReport[reportID]
			k := key{account.AccountID, raw.AssetSymbol}
			quantity := amounts[k]
			if quantity == nil {
				continue
			}
			delete(amounts, k)
			assetID, err := uuid.FromBytes(raw.AssetID)
			if err != nil {
				return err
			}
			row, err := balanceRow(ctx, book, account, &sqlcgen.ListBalancesByReportRow{AssetID: raw.AssetID, AssetSymbol: raw.AssetSymbol, Equity: raw.Equity}, decisions[reportID][assetID])
			if err != nil {
				return err
			}
			if row.Excluded {
				continue
			}
			if quantity.Sign() == 0 {
				total.add(decimal.Zero())
			} else if row.PriceUSD != nil {
				total.add(decimal.Mul(quantity, decimal.ParseOrZero(*row.PriceUSD)))
			} else {
				total.completeness = valuation.CompletenessIncomplete
				total.issues = append(total.issues, valuation.Issue{Code: "no_price", AccountID: &account.AccountID, AccountName: account.AccountName, Symbol: row.AssetSymbol, ReportObservedAt: account.ReportObservedAt, Detail: "The product holding has no known USD value and is omitted from the filtered total."})
			}
		}
	}
	if len(amounts) != 0 {
		return errors.New("product holding lacks a source balance")
	}
	return nil
}
