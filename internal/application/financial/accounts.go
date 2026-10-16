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

func (s *Service) accountView(ctx context.Context, p PortfolioContext, accountID uuid.UUID, cycleID *uuid.UUID) (AccountView, *uuid.UUID, error) {
	account, err := mysql.Queries(ctx, s.db).GetAccount(ctx, sqlcgen.GetAccountParams{PortfolioID: p.ID[:], ID: accountID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return AccountView{}, nil, valuation.ErrNotFound
	}
	if err != nil {
		return AccountView{}, nil, err
	}
	q := mysql.Queries(ctx, s.db)
	var point sqlcgen.ValuationPoint
	if cycleID == nil {
		row, readErr := q.GetLatestValuationPoint(ctx, p.ID[:])
		point, err = sqlcgen.ValuationPoint(row), readErr
	} else {
		point, err = q.GetValuationPoint(ctx, sqlcgen.GetValuationPointParams{PortfolioID: p.ID[:], CycleID: cycleID[:]})
		if errors.Is(err, sql.ErrNoRows) {
			return AccountView{}, nil, valuation.ErrNotFound
		}
		if err == nil {
			available, readErr := q.IsPointInRetainedHistory(ctx, sqlcgen.IsPointInRetainedHistoryParams{PortfolioID: p.ID[:], CycleID: cycleID[:]})
			if readErr != nil {
				return AccountView{}, nil, readErr
			}
			if !available {
				latest, readErr := q.GetLatestValuationPoint(ctx, p.ID[:])
				if readErr != nil {
					return AccountView{}, nil, readErr
				}
				id, readErr := uuid.FromBytes(latest.CycleID)
				if readErr != nil {
					return AccountView{}, nil, readErr
				}
				if id != *cycleID {
					return AccountView{}, nil, valuation.ErrNotFound
				}
			}
		}
	}
	view := AccountView{Context: Context{PortfolioID: p.ID, Timezone: p.Timezone}, Issues: []valuation.Issue{}}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return AccountView{}, nil, err
	}
	if err == nil {
		if point.DetailState == "expired" || point.DetailState == "pruning" {
			return AccountView{}, nil, valuation.ErrDetailExpired
		}
		id, readErr := uuid.FromBytes(point.CycleID)
		if readErr != nil {
			return AccountView{}, nil, readErr
		}
		view.Context.CycleID, view.Context.EvaluatedAt = &id, &point.EvaluatedAt
		view.Account, err = s.valuation.Account(ctx, p.ID, id, accountID)
		if err == nil {
			view.Issues = valuation.AccountIssues(&view.Account)
			return view, view.Account.SourceReportID, nil
		}
		if !errors.Is(err, valuation.ErrNotFound) {
			return AccountView{}, nil, err
		}
		if cycleID != nil && (account.AccountType != string(adapters.TypeManual) || account.IdentityArchivedAt.Valid) {
			return AccountView{}, nil, valuation.ErrNotFound
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
	if cycleID != nil {
		return view, nil, nil
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

func staleAccountIssue(account *valuation.AccountValuation) valuation.Issue {
	for _, issue := range valuation.AccountIssues(account) {
		if issue.Code == "stale" {
			return issue
		}
	}
	return valuation.Issue{Code: "stale", AccountID: &account.AccountID, AccountName: account.AccountName, Detail: "An earlier successful account report is carried forward."}
}

// AccountReport resolves one point and source report for every holding kind.
// Current manual edits are an explicit overlay, never historical observations.
func (s *Service) AccountReport(ctx context.Context, p PortfolioContext, accountID uuid.UUID, cycleID *uuid.UUID, includePendingManual bool) (AccountView, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (AccountView, error) {
		view, source, err := s.accountView(ctx, p, accountID, cycleID)
		if err != nil {
			return AccountView{}, err
		}
		view, err = s.readAccountBalances(ctx, p, view, source, includePendingManual)
		if err != nil {
			return AccountView{}, err
		}
		switch view.Account.AccountType {
		case "earn":
			return s.readAccountEarn(ctx, p, view)
		case "dual_investment":
			return s.readAccountDual(ctx, p, view, source)
		case "manual":
			return view, nil
		default:
			if positionAccount(&view.Account) {
				return s.readAccountPositions(ctx, p, view, source)
			}
			return view, nil
		}
	})
}
