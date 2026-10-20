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

type EarnList = FlatList[EarnPositionRow]

func (s *Service) readEarnReport(ctx context.Context, p PortfolioContext, account *valuation.AccountValuation) ([]EarnPositionRow, bool, error) {
	if account.AccountType != "earn" || account.SourceReportID == nil {
		return []EarnPositionRow{}, false, nil
	}
	items, err := s.readEarnReports(ctx, p, []valuation.AccountValuation{*account})
	return items[account.AccountID], err == nil, err
}

func (s *Service) readEarnReports(ctx context.Context, p PortfolioContext, accounts []valuation.AccountValuation) (map[uuid.UUID][]EarnPositionRow, error) {
	out := map[uuid.UUID][]EarnPositionRow{}
	ids := [][]byte{}
	byReport := map[uuid.UUID]*valuation.AccountValuation{}
	for i := range accounts {
		a := &accounts[i]
		if a.AccountType != "earn" || a.SourceReportID == nil {
			continue
		}
		ids = append(ids, a.SourceReportID[:])
		byReport[*a.SourceReportID] = a
		out[a.AccountID] = []EarnPositionRow{}
	}
	for start := 0; start < len(ids); start += 500 {
		batch := ids[start:min(start+500, len(ids))]
		balances, err := mysql.Queries(ctx, s.db).ListBalancesByReports(ctx, sqlcgen.ListBalancesByReportsParams{PortfolioID: p.ID[:], ReportIds: batch})
		if err != nil {
			return nil, err
		}
		symbols := map[uuid.UUID]map[string]string{}
		decisions := map[uuid.UUID]map[uuid.UUID]*valuation.BalanceValuation{}
		for _, raw := range batch {
			id, err := uuid.FromBytes(raw)
			if err != nil {
				return nil, err
			}
			symbols[id] = map[string]string{}
			decisions[id] = balanceDecisions(byReport[id])
		}
		for _, b := range balances {
			id, err := uuid.FromBytes(b.ReportID)
			if err != nil {
				return nil, err
			}
			assetID, err := uuid.FromBytes(b.AssetID)
			if err != nil {
				return nil, err
			}
			if v := decisions[id][assetID]; v != nil {
				symbols[id][b.AssetSymbol] = v.CanonicalAssetSymbol
			}
		}
		rows, err := mysql.Queries(ctx, s.db).ListEarnPositionsByReports(ctx, sqlcgen.ListEarnPositionsByReportsParams{PortfolioID: p.ID[:], ReportIds: batch})
		if err != nil {
			return nil, err
		}
		for i := range rows {
			id, err := uuid.FromBytes(rows[i].ReportID)
			if err != nil {
				return nil, err
			}
			account := byReport[id]
			row, err := earnPositionRow(account, &rows[i], symbols[id])
			if err != nil {
				return nil, err
			}
			out[account.AccountID] = append(out[account.AccountID], row)
		}
	}
	return out, nil
}

func earnPositionRow(account *valuation.AccountValuation, stored *sqlcgen.EarnPosition, canonicalAssetSymbols map[string]string) (EarnPositionRow, error) {
	var details adapters.EarnDetails
	if err := json.Unmarshal(stored.DetailsJson, &details); err != nil {
		return EarnPositionRow{}, err
	}
	position := adapters.EarnPosition{PositionKey: stored.PositionKey, ProductRef: nullStringPtr(stored.ProductRef), ProductName: nullStringPtr(stored.ProductName), Type: stored.ProductType, Granularity: stored.Granularity, AmountBasis: stored.AmountBasis, Status: nullStringPtr(stored.Status), SubscribedAt: earnTime(stored.SubscribedAt), MaturityAt: earnTime(stored.MaturityAt), Assets: details.Assets, Rates: details.Rates, Rewards: details.Rewards, Details: details.Details, Problems: details.Issues}
	if err := adapters.NormalizeEarnPosition(&position); err != nil {
		return EarnPositionRow{}, err
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
	return row, nil
}

func (s *Service) AccountEarnPositions(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	return s.AccountEarnPositionsAt(ctx, p, accountID, nil)
}

func (s *Service) AccountEarnPositionsAt(ctx context.Context, p PortfolioContext, accountID uuid.UUID, cycleID *uuid.UUID) (AccountView, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (AccountView, error) {
		view, _, err := s.accountView(ctx, p, accountID, cycleID)
		if err != nil {
			return AccountView{}, err
		}
		return s.readAccountEarn(ctx, p, view)
	})
}

func (s *Service) readAccountEarn(ctx context.Context, p PortfolioContext, view AccountView) (AccountView, error) {
	var err error
	view.EarnPositions, _, err = s.readEarnReport(ctx, p, &view.Account)
	if err != nil {
		return AccountView{}, err
	}
	sortEarnPositions(view.EarnPositions, map[uuid.UUID]int{view.Account.AccountID: 0})
	for _, row := range view.EarnPositions {
		view.Issues = appendNonStaleIssues(view.Issues, row.Issues)
	}
	return view, nil
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
		c, point, accounts, err := s.contextAt(ctx, p, f.CycleID)
		if err != nil {
			return EarnList{}, err
		}
		out := EarnList{
			Context:      c,
			Items:        []EarnPositionRow{},
			Offset:       offset,
			Limit:        limit,
			Completeness: valuation.CompletenessIncomplete,
			Issues:       []valuation.Issue{},
		}
		out.Coverage = reportCoverage(accounts, f, "earn", point != nil)
		total := newHoldingTotal(accounts, f, "earn", point != nil)
		out.setTotal(total)
		if point == nil {
			out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No collection cycle has produced a valuation point yet."})
			return out, nil
		}
		out.Completeness = point.Completeness()
		out.Issues = append(out.Issues, point.Issues...)
		matching := []valuation.AccountValuation{}
		for _, account := range accounts {
			if matchesAccount(f, &account) && account.AccountType == "earn" {
				matching = append(matching, account)
			}
		}
		reports, err := s.readEarnReports(ctx, p, matching)
		if err != nil {
			return EarnList{}, err
		}
		all := []EarnPositionRow{}
		holdings := []productHolding{}
		for i := range accounts {
			account := &accounts[i]
			if account.AccountType != "earn" || !matchesAccount(f, account) {
				continue
			}
			rows := reports[account.AccountID]
			for _, row := range rows {
				if !matchesEarnAssets(f, row.Assets) {
					continue
				}
				all = append(all, row)
				for _, asset := range row.Assets {
					holdings = append(holdings, productHolding{row.AccountID, asset.AssetSymbol, asset.Amount})
				}
				out.Issues = appendNonStaleIssues(out.Issues, row.Issues)
			}

		}
		if err := s.totalProductHoldings(ctx, p, matching, holdings, total); err != nil {
			return EarnList{}, err
		}
		out.setTotal(total)
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
