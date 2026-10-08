package financial

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func (s *Service) accountView(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, *uuid.UUID, error) {
	account, err := s.q.GetAccount(ctx, sqlcgen.GetAccountParams{PortfolioID: p.ID[:], ID: accountID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return AccountView{}, nil, valuation.ErrNotFound
	}
	if err != nil {
		return AccountView{}, nil, err
	}
	c, point, accounts, err := s.latestContext(ctx, p)
	if err != nil {
		return AccountView{}, nil, err
	}
	view := AccountView{Context: c, Issues: []valuation.Issue{}}
	if point != nil {
		for i := range accounts {
			if accounts[i].AccountID == accountID {
				view.Account = accounts[i]
				view.Issues = valuation.AccountIssues(&view.Account)
				return view, view.Account.SourceReportID, nil
			}
		}
	}
	identityID, err := uuid.FromBytes(account.IdentityID)
	if err != nil {
		return AccountView{}, nil, err
	}
	view.Account = valuation.AccountValuation{
		AccountID: accountID, AccountName: account.DisplayName, AccountType: account.AccountType, Selected: account.Selected,
		IdentityID: identityID, IdentityName: account.IdentityName, IdentityStatus: account.IdentityStatus, Platform: account.Platform,
		AccountStructure: account.AccountStructure,
		Status:           "failed", Method: valuation.MethodMissing, Freshness: valuation.FreshnessStale, Omitted: nil,
	}
	view.Issues = append(view.Issues, valuation.Issue{Code: "outside_boundary", AccountID: &accountID, AccountName: account.DisplayName, Detail: "The account is not part of the latest valuation point; the last successful report is shown without a value."})
	src, err := s.q.GetLatestOkReportForAccount(ctx, sqlcgen.GetLatestOkReportForAccountParams{PortfolioID: p.ID[:], AccountID: accountID[:], ScopeVersion: adapters.ReportScope(adapters.Platform(account.Platform), adapters.AccountType(account.AccountType)), Before: s.now().UTC().Add(time.Hour)})
	if errors.Is(err, sql.ErrNoRows) {
		return view, nil, nil
	}
	if err != nil {
		return AccountView{}, nil, err
	}
	sourceID, err := uuid.FromBytes(src.ID)
	if err != nil {
		return AccountView{}, nil, err
	}
	observed := src.ObservedAt.Time
	view.Account.Status = "ok"
	view.Account.SourceReportID = &sourceID
	view.Account.ReportObservedAt = &observed
	view.Account.SourceMarginMode = nullStringPtr(src.MarginMode)
	view.Account.Equity, view.Account.AdjustedEquity, view.Account.AvailableEquity = nullDecimal(src.Equity), nullDecimal(src.AdjustedEquity), nullDecimal(src.AvailableEquity)
	view.Account.InitialMarginRatio, view.Account.MaintenanceMarginRatio, view.Account.UnifiedMarginRatio = nullDecimal(src.InitialMarginRatio), nullDecimal(src.MaintenanceMarginRatio), nullDecimal(src.UnifiedMarginRatio)
	return view, &sourceID, nil
}

func (s *Service) positionView(ctx context.Context, p PortfolioContext, accountID uuid.UUID, pointID *uuid.UUID) (AccountView, *uuid.UUID, error) {
	if pointID == nil {
		return s.accountView(ctx, p, accountID)
	}
	detail, err := s.Detail(ctx, p, *pointID)
	if err != nil {
		return AccountView{}, nil, err
	}
	for i := range detail.Accounts {
		account := &detail.Accounts[i]
		if account.AccountID == accountID {
			view := AccountView{Context: Context{PortfolioID: p.ID, Timezone: p.Timezone, PointID: &detail.Point.ID, JobID: &detail.Point.JobID, EvaluatedAt: &detail.Point.EvaluatedAt}, Account: *account, Issues: positionAccountIssues(account)}
			return view, account.SourceReportID, nil
		}
	}
	return AccountView{}, nil, valuation.ErrNotFound
}

func staleAccountIssue(account *valuation.AccountValuation) valuation.Issue {
	for _, issue := range valuation.AccountIssues(account) {
		if issue.Code == "stale" {
			return issue
		}
	}
	return valuation.Issue{Code: "stale", AccountID: &account.AccountID, AccountName: account.AccountName, Detail: "An earlier successful Account Report is carried forward."}
}
