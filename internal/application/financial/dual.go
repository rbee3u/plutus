package financial

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// DualPositionRow describes principal already included in the Account's Balances.
type DualPositionRow struct {
	adapters.DualPosition
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
		DualPosition: adapters.DualPosition{
			PlatformPositionRef: p.PlatformPositionRef,
			ProductRef:          nullStringPtr(p.ProductRef),
			OrderRef:            nullStringPtr(p.OrderRef),
			BaseSymbol:          nullStringPtr(p.BaseSymbol),
			QuoteSymbol:         nullStringPtr(p.QuoteSymbol),
			InvestSymbol:        p.InvestSymbol,
			Principal:           p.Principal,
			Direction:           nullStringPtr(p.Direction),
			TargetPrice:         nullDecimal(p.TargetPrice),
			AnnualizedRate:      nullDecimal(p.AnnualizedRate),
			RateKind:            nullStringPtr(p.RateKind),
			Status:              p.Status,
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
	return []valuation.Issue{{Code: "dual_terms_missing", AccountID: &row.AccountID, AccountName: row.AccountName, Symbol: row.InvestSymbol, Detail: "Dual position " + row.PlatformPositionRef + " has no " + strings.Join(missing, ", ") + ". Its known principal is retained."}}
}

// AccountDualPositions uses the same source report as AccountBalances, including stale reads.
func (s *Service) AccountDualPositions(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	view, source, err := s.accountView(ctx, p, accountID)
	if err != nil {
		return AccountView{}, err
	}
	view.DualPositions = []DualPositionRow{}
	if source == nil {
		return view, nil
	}
	rows, err := s.q.ListDualPositionsByReport(ctx, source[:])
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

// FlatDualPositions lists only subscriptions in selected Accounts at the latest point.
func (s *Service) FlatDualPositions(ctx context.Context, p PortfolioContext, f *ListFilter) (FlatList[DualPositionRow], error) {
	limit, offset, err := normalizePage(f.Limit, f.Offset)
	if err != nil {
		return FlatList[DualPositionRow]{}, err
	}
	c, point, accounts, err := s.latestContext(ctx, p)
	if err != nil {
		return FlatList[DualPositionRow]{}, err
	}
	out := FlatList[DualPositionRow]{Context: c, Items: []DualPositionRow{}, Offset: offset, Limit: limit, Completeness: valuation.CompletenessIncomplete, Issues: []valuation.Issue{}}
	if point == nil {
		out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
		return out, nil
	}
	out.Completeness = point.Completeness()
	out.Issues = append(out.Issues, point.Issues...)
	byAccount := indexAccounts(accounts)
	rows, err := s.q.ListDualPositionsByJob(ctx, sqlcgen.ListDualPositionsByJobParams{PortfolioID: p.ID[:], JobID: point.JobID[:]})
	if err != nil {
		return out, err
	}
	var all []DualPositionRow
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
		if !matchesSymbol(f.Symbol, row.InvestSymbol, optionalSymbol(row.BaseSymbol), optionalSymbol(row.QuoteSymbol)) {
			continue
		}
		all = append(all, row)
		out.Issues = append(out.Issues, dualIssues(&row)...)
	}
	sortDualPositions(all, accountRanks(accounts))
	out.paginate(all)
	return out, nil
}
