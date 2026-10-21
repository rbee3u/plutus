package financial

import (
	"context"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

// DualPositionRow describes principal already included in the account's balances.
type DualPositionRow struct {
	adapter.DualPosition
	AccountID        uuid.UUID
	AccountName      string
	AccountType      string
	IdentityID       uuid.UUID
	IdentityName     string
	Platform         string
	Freshness        string
	ReportObservedAt *time.Time
	Issues           []valuation.Issue
}

func dualPositionRow(account *valuation.AccountValuation, p *sqlcgen.DualPosition) DualPositionRow {
	row := DualPositionRow{
		AccountID: account.AccountID, AccountName: account.AccountName, AccountType: account.AccountType,
		IdentityID: account.IdentityID, IdentityName: account.IdentityName, Platform: account.Platform,
		Freshness: account.Freshness, ReportObservedAt: account.ReportObservedAt,
		DualPosition: adapter.DualPosition{
			PositionKey:          p.PositionKey,
			ProductRef:           nullStringPtr(p.ProductRef),
			OrderRef:             nullStringPtr(p.OrderRef),
			BaseAssetSymbol:      nullStringPtr(p.BaseAssetSymbol),
			QuoteAssetSymbol:     nullStringPtr(p.QuoteAssetSymbol),
			PrincipalAssetSymbol: p.PrincipalAssetSymbol,
			Principal:            p.Principal,
			Direction:            nullStringPtr(p.Direction),
			TargetPrice:          nullDecimal(p.TargetPrice),
			AnnualizedRate:       nullDecimal(p.AnnualizedRate),
			RateKind:             nullStringPtr(p.RateKind),
			Status:               p.Status,
		},
	}
	if p.ExpiryAt.Valid {
		expiry := p.ExpiryAt.Time
		row.ExpiryAt = &expiry
	}
	if p.SubscribedAt.Valid {
		subscribed := p.SubscribedAt.Time
		row.SubscribedAt = &subscribed
	}
	row.Issues = dualIssues(&row)
	if account.Freshness == valuation.FreshnessStale {
		row.Issues = append(row.Issues, staleAccountIssue(account))
	}
	return row
}

func dualIssues(row *DualPositionRow) []valuation.Issue {
	missing := row.MissingFields()
	if len(missing) == 0 {
		return nil
	}
	return []valuation.Issue{{Code: "dual_terms_missing", AccountID: &row.AccountID, AccountName: row.AccountName, Symbol: row.PrincipalAssetSymbol, Detail: "Dual position " + row.PositionKey + " has no " + strings.Join(missing, ", ") + ". Its known principal is retained."}}
}

// AccountDualPositions uses the same source report as AccountBalances, including stale reads.
func (s *Service) AccountDualPositions(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	return s.AccountDualPositionsAt(ctx, p, accountID, nil)
}

func (s *Service) AccountDualPositionsAt(ctx context.Context, p PortfolioContext, accountID uuid.UUID, cycleID *uuid.UUID) (AccountView, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (AccountView, error) {
		view, source, err := s.accountView(ctx, p, accountID, cycleID)
		if err != nil {
			return AccountView{}, err
		}
		return s.readAccountDual(ctx, p, view, source)
	})
}

func (s *Service) readAccountDual(ctx context.Context, p PortfolioContext, view AccountView, source *uuid.UUID) (AccountView, error) {
	view.DualPositions = []DualPositionRow{}
	if source == nil {
		return view, nil
	}
	rows, err := mysql.Queries(ctx, s.db).ListDualPositionsByReport(ctx, sqlcgen.ListDualPositionsByReportParams{PortfolioID: p.ID[:], ReportID: source[:]})
	if err != nil {
		return AccountView{}, err
	}
	for i := range rows {
		row := dualPositionRow(&view.Account, &rows[i])
		view.DualPositions = append(view.DualPositions, row)
		view.Issues = append(view.Issues, dualIssues(&row)...)
	}
	return view, nil
}

// FlatDualPositions lists only subscriptions in selected accounts at the latest point.
func (s *Service) FlatDualPositions(ctx context.Context, p PortfolioContext, f *ListFilter) (FlatList[DualPositionRow], error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (FlatList[DualPositionRow], error) {
		limit, offset, err := normalizePage(f.Limit, f.Offset)
		if err != nil {
			return FlatList[DualPositionRow]{}, err
		}
		c, point, accounts, err := s.contextAt(ctx, p, f.CycleID)
		if err != nil {
			return FlatList[DualPositionRow]{}, err
		}
		out := FlatList[DualPositionRow]{Context: c, Items: []DualPositionRow{}, Offset: offset, Limit: limit, Completeness: valuation.CompletenessIncomplete, Issues: []valuation.Issue{}}
		out.Coverage = reportCoverage(accounts, f, "dual_investment", point != nil)
		total := newHoldingTotal(accounts, f, "dual_investment", point != nil)
		out.setTotal(total)
		if point == nil {
			out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No collection cycle has produced a valuation point yet."})
			return out, nil
		}
		out.Completeness = point.Completeness()
		out.Issues = append(out.Issues, point.Issues...)
		byAccount := indexAccounts(accounts)
		rows, err := mysql.Queries(ctx, s.db).ListDualPositionsByCycle(ctx, sqlcgen.ListDualPositionsByCycleParams{PortfolioID: p.ID[:], CycleID: point.CycleID[:]})
		if err != nil {
			return out, err
		}
		var all []DualPositionRow
		holdings := []productHolding{}
		matching := []valuation.AccountValuation{}
		for _, account := range accounts {
			if account.AccountType == "dual_investment" && matchesAccount(f, &account) {
				matching = append(matching, account)
			}
		}
		for i := range rows {
			r := &rows[i]
			id, err := uuid.FromBytes(r.AccountID)
			if err != nil {
				return out, err
			}
			account := byAccount[id]
			if account == nil || !matchesAccount(f, account) {
				continue
			}
			row := dualPositionRow(account, &r.DualPosition)
			if !matchesSymbol(f.Symbol, row.PrincipalAssetSymbol, optionalSymbol(row.BaseAssetSymbol), optionalSymbol(row.QuoteAssetSymbol)) {
				continue
			}
			all = append(all, row)
			holdings = append(holdings, productHolding{row.AccountID, row.PrincipalAssetSymbol, row.Principal})
			out.Issues = append(out.Issues, dualIssues(&row)...)
		}
		if err := s.totalProductHoldings(ctx, p, matching, holdings, total); err != nil {
			return out, err
		}
		out.setTotal(total)
		sortDualPositions(all, accountRanks(accounts))
		out.paginate(all)
		return out, nil
	})
}
