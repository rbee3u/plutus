package financial

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

type EarnPositionRow struct {
	adapters.EarnPosition
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

type EarnList struct {
	FlatList[EarnPositionRow]
	DetailsComplete bool
}

func (s *Service) readEarnReport(ctx context.Context, p PortfolioContext, account *valuation.AccountValuation) ([]EarnPositionRow, bool, error) {
	items := []EarnPositionRow{}
	if account.AccountType != "earn" || account.SourceReportID == nil {
		return items, false, nil
	}
	id := *account.SourceReportID
	balances, err := mysql.Queries(ctx, s.db).ListBalancesByReport(ctx, sqlcgen.ListBalancesByReportParams{PortfolioID: p.ID[:], ReportID: id[:]})
	if err != nil {
		return nil, false, err
	}
	canonicalAssetSymbols := map[string]string{}
	for _, balance := range balances {
		for _, decision := range account.BalanceValuations {
			if string(decision.AssetID[:]) == string(balance.AssetID) {
				canonicalAssetSymbols[balance.AssetSymbol] = decision.CanonicalAssetSymbol
				break
			}
		}
	}
	rows, err := mysql.Queries(ctx, s.db).ListEarnPositionsByReport(ctx, sqlcgen.ListEarnPositionsByReportParams{PortfolioID: p.ID[:], ReportID: id[:]})
	if err != nil {
		return nil, false, err
	}
	for i := range rows {
		var details adapters.EarnDetails
		if err := json.Unmarshal(rows[i].DetailsJson, &details); err != nil {
			return nil, false, err
		}
		stored := rows[i]
		position := adapters.EarnPosition{PositionKey: stored.PositionKey, ProductRef: nullStringPtr(stored.ProductRef), ProductName: nullStringPtr(stored.ProductName), Type: stored.ProductType, Granularity: stored.Granularity, AmountBasis: stored.AmountBasis, Status: nullStringPtr(stored.Status), SubscribedAt: earnTime(stored.SubscribedAt), MaturityAt: earnTime(stored.MaturityAt), Assets: details.Assets, Rates: details.Rates, Rewards: details.Rewards, Details: details.Details, Problems: details.Issues}
		if err := adapters.NormalizeEarnPosition(&position); err != nil {
			return nil, false, err
		}
		for i := range position.Assets {
			asset := &position.Assets[i]
			asset.CanonicalAssetSymbol = adapters.OptionalText(canonicalAssetSymbols[asset.AssetSymbol])
		}
		slices.SortFunc(position.Assets, compareEarnAssets)
		row := EarnPositionRow{
			EarnPosition:     position,
			AccountID:        account.AccountID,
			AccountName:      account.AccountName,
			AccountType:      account.AccountType,
			IdentityID:       account.IdentityID,
			IdentityName:     account.IdentityName,
			Platform:         account.Platform,
			Freshness:        account.Freshness,
			ReportObservedAt: account.ReportObservedAt,
			Issues:           []valuation.Issue{},
		}
		for _, problem := range position.Problems {
			row.Issues = append(row.Issues, valuation.Issue{Code: "earn_metadata_unavailable", AccountID: &row.AccountID, AccountName: row.AccountName, Detail: "Earn position " + row.PositionKey + ": " + problem})
		}
		if account.Freshness == valuation.FreshnessStale {
			row.Issues = append(row.Issues, staleAccountIssue(account))
		}
		items = append(items, row)
	}
	return items, true, nil
}

func (s *Service) AccountEarnPositions(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (AccountView, error) {
		view, _, err := s.accountView(ctx, p, accountID)
		if err != nil {
			return AccountView{}, err
		}
		view.EarnPositions, _, err = s.readEarnReport(ctx, p, &view.Account)
		if err != nil {
			return AccountView{}, err
		}
		sortEarnPositions(view.EarnPositions, map[uuid.UUID]int{accountID: 0})
		for _, row := range view.EarnPositions {
			view.Issues = appendNonStaleIssues(view.Issues, row.Issues)
		}
		return view, nil
	})
}

func matchesEarnAssets(f *ListFilter, assets []adapters.EarnAmount) bool {
	if f.Symbol == "" {
		return true
	}
	for _, asset := range assets {
		if matchesSymbol(f.Symbol, asset.AssetSymbol, optionalSymbol(asset.CanonicalAssetSymbol)) {
			return true
		}
	}
	return false
}

func (s *Service) FlatEarnPositions(ctx context.Context, p PortfolioContext, f *ListFilter) (EarnList, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (EarnList, error) {
		limit, offset, err := normalizePage(f.Limit, f.Offset)
		if err != nil {
			return EarnList{}, err
		}
		c, point, accounts, err := s.latestContext(ctx, p)
		if err != nil {
			return EarnList{}, err
		}
		out := EarnList{
			FlatList: FlatList[EarnPositionRow]{
				Context:      c,
				Items:        []EarnPositionRow{},
				Offset:       offset,
				Limit:        limit,
				Completeness: valuation.CompletenessIncomplete,
				Issues:       []valuation.Issue{},
			},
		}
		if point == nil {
			out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
			return out, nil
		}
		out.Completeness = point.Completeness()
		out.DetailsComplete = true
		out.Issues = append(out.Issues, point.Issues...)
		all := []EarnPositionRow{}
		for i := range accounts {
			account := &accounts[i]
			if account.AccountType != "earn" || !matchesAccount(f, account) {
				continue
			}
			rows, collected, err := s.readEarnReport(ctx, p, account)
			if err != nil {
				return EarnList{}, err
			}
			if !collected {
				out.DetailsComplete = false
			}
			for _, row := range rows {
				if !matchesEarnAssets(f, row.Assets) {
					continue
				}
				all = append(all, row)
				out.Issues = appendNonStaleIssues(out.Issues, row.Issues)
			}

		}
		sortEarnPositions(all, accountRanks(accounts))
		out.paginate(all)
		return out, nil
	})
}

// Account-level stale issues are already present in the enclosing view or list.
func appendNonStaleIssues(out, issues []valuation.Issue) []valuation.Issue {
	for _, issue := range issues {
		if issue.Code != "stale" {
			out = append(out, issue)
		}
	}
	return out
}

func earnTime(at sql.NullTime) *time.Time {
	if !at.Valid {
		return nil
	}
	return &at.Time
}
