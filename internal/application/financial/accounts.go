package financial

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func (s *Service) accountView(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, *uuid.UUID, error) {
	account, err := mysql.Queries(ctx, s.db).GetAccount(ctx, sqlcgen.GetAccountParams{PortfolioID: p.ID[:], ID: accountID[:]})
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
		AccountID: accountID, AccountName: adapters.AccountName(adapters.AccountType(account.AccountType)), AccountType: account.AccountType, Selected: account.Selected,
		IdentityID: identityID, IdentityName: account.IdentityName, IdentityStatus: "enabled", Platform: account.Platform,
		AccountStructure: account.AccountStructure,
		Status:           "failed", Method: valuation.MethodMissing, Freshness: valuation.FreshnessStale, Omitted: nil,
	}
	if account.IdentityArchivedAt.Valid {
		view.Account.IdentityStatus = "archived"
	}
	if account.AccountType != string(adapters.TypeManual) || account.IdentityArchivedAt.Valid {
		view.Issues = append(view.Issues, valuation.Issue{Code: "outside_boundary", AccountID: &accountID, AccountName: adapters.AccountName(adapters.AccountType(account.AccountType)), Detail: "The account is not part of the latest valuation point; the last successful report is shown without a value."})
	}
	src, err := mysql.Queries(ctx, s.db).GetLatestSucceededReportForAccount(ctx, sqlcgen.GetLatestSucceededReportForAccountParams{PortfolioID: p.ID[:], AccountID: accountID[:], Before: s.now().UTC().Add(time.Hour)})
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
	observed := src.CollectedAt.Time
	if src.SourceObservedAt.Valid {
		observed = src.SourceObservedAt.Time
	}
	view.Account.Status = adapters.StatusSucceeded
	view.Account.SourceReportID = &sourceID
	view.Account.ReportObservedAt = &observed
	view.Account.SourceMarginMode = nullStringPtr(src.MarginMode)
	view.Account.Equity, view.Account.AdjustedEquity, view.Account.AvailableEquity = nullDecimal(src.ReportedEquityUsd), nullDecimal(src.AdjustedEquityUsd), nullDecimal(src.AvailableEquityUsd)
	view.Account.InitialMarginRatio = valuation.MarginRatio(src.ReportedInitialMarginRatio, src.InitialMarginRequirementUsd, src.AdjustedEquityUsd)
	view.Account.MaintenanceMarginRatio = valuation.MarginRatio(src.ReportedMaintenanceMarginRatio, src.MaintenanceMarginRequirementUsd, src.AdjustedEquityUsd)
	view.Account.MarginRatio = nullDecimal(src.ReportedMarginRatio)
	return view, &sourceID, nil
}

func (s *Service) positionView(ctx context.Context, p PortfolioContext, accountID uuid.UUID, cycleID *uuid.UUID) (AccountView, *uuid.UUID, error) {
	if cycleID == nil {
		return s.accountView(ctx, p, accountID)
	}
	detail, err := s.Detail(ctx, p, *cycleID)
	if err != nil {
		return AccountView{}, nil, err
	}
	if detail.Point.DetailState == "expired" {
		return AccountView{}, nil, valuation.ErrDetailExpired
	}
	for i := range detail.Accounts {
		account := &detail.Accounts[i]
		if account.AccountID == accountID {
			view := AccountView{Context: Context{PortfolioID: p.ID, Timezone: p.Timezone, ValuationRevision: new(detail.Point.Revision()), CycleID: &detail.Point.CycleID, EvaluatedAt: &detail.Point.EvaluatedAt}, Account: *account, Issues: positionAccountIssues(account)}
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
