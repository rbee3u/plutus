package financial

import (
	"context"
	"encoding/json"
	"slices"
	"time"

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

// EarnBalanceSummary explicitly represents a source report without product detail.
type EarnBalanceSummary struct {
	AccountID        uuid.UUID
	AccountName      string
	IdentityName     string
	Platform         string
	Assets           []adapters.EarnAmount
	Freshness        string
	ReportObservedAt *time.Time
	Issues           []valuation.Issue
}

type EarnList struct {
	FlatList[EarnPositionRow]
	BalanceSummaries []EarnBalanceSummary
	DetailsComplete  bool
}

func (s *Service) readEarnReport(ctx context.Context, p PortfolioContext, account *valuation.AccountValuation) ([]EarnPositionRow, []EarnBalanceSummary, bool, error) {
	items := []EarnPositionRow{}
	summaries := []EarnBalanceSummary{}
	if account.AccountType != "earn" || account.SourceReportID == nil {
		return items, summaries, false, nil
	}
	id := *account.SourceReportID
	collected, err := s.q.GetEarnDetailsCollected(ctx, sqlcgen.GetEarnDetailsCollectedParams{PortfolioID: p.ID[:], ID: id[:]})
	if err != nil {
		return nil, nil, false, err
	}
	balances, err := s.q.ListBalancesByReport(ctx, id[:])
	if err != nil {
		return nil, nil, false, err
	}
	assetSymbols := map[string]string{}
	for _, balance := range balances {
		assetSymbols[balance.PlatformAssetSymbol] = balance.AssetSymbol
	}
	if !collected {
		issue := valuation.Issue{
			Code:        "earn_details_unavailable",
			AccountID:   &account.AccountID,
			AccountName: account.AccountName,
			Detail:      "Earn product details were not collected in this Account Report. Known Balance amounts are shown separately.",
		}
		summary := EarnBalanceSummary{
			AccountID:        account.AccountID,
			AccountName:      account.AccountName,
			IdentityName:     account.IdentityName,
			Platform:         account.Platform,
			Freshness:        account.Freshness,
			ReportObservedAt: account.ReportObservedAt,
			Assets:           []adapters.EarnAmount{},
			Issues:           []valuation.Issue{issue},
		}
		for _, balance := range balances {
			summary.Assets = append(summary.Assets, adapters.EarnAmount{PlatformAssetSymbol: balance.PlatformAssetSymbol, Amount: balance.Equity, AssetSymbol: adapters.OptionalText(balance.AssetSymbol)})
		}
		if account.Freshness == valuation.FreshnessStale {
			summary.Issues = append(summary.Issues, staleAccountIssue(account))
		}
		return items, append(summaries, summary), false, nil
	}
	rows, err := s.q.ListEarnPositionsByReport(ctx, sqlcgen.ListEarnPositionsByReportParams{PortfolioID: p.ID[:], ReportID: id[:]})
	if err != nil {
		return nil, nil, false, err
	}
	for i := range rows {
		var position adapters.EarnPosition
		if err := json.Unmarshal(rows[i].ObservationJson, &position); err != nil {
			return nil, nil, false, err
		}
		if err := adapters.NormalizeEarnPosition(&position); err != nil {
			return nil, nil, false, err
		}
		for i := range position.Assets {
			asset := &position.Assets[i]
			asset.AssetSymbol = adapters.OptionalText(assetSymbols[asset.PlatformAssetSymbol])
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
			row.Issues = append(row.Issues, valuation.Issue{Code: "earn_metadata_unavailable", AccountID: &row.AccountID, AccountName: row.AccountName, Detail: "Earn position " + row.PlatformPositionRef + ": " + problem})
		}
		if account.Freshness == valuation.FreshnessStale {
			row.Issues = append(row.Issues, staleAccountIssue(account))
		}
		items = append(items, row)
	}
	return items, summaries, true, nil
}

func (s *Service) AccountEarnPositions(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	view, _, err := s.accountView(ctx, p, accountID)
	if err != nil {
		return AccountView{}, err
	}
	view.EarnPositions, view.EarnBalanceSummaries, view.EarnDetailsCollected, err = s.readEarnReport(ctx, p, &view.Account)
	if err != nil {
		return AccountView{}, err
	}
	sortEarnPositions(view.EarnPositions, map[uuid.UUID]int{accountID: 0})
	for _, row := range view.EarnPositions {
		view.Issues = appendNonStaleIssues(view.Issues, row.Issues)
	}
	for _, summary := range view.EarnBalanceSummaries {
		view.Issues = appendNonStaleIssues(view.Issues, summary.Issues)
	}
	return view, nil
}

func matchesEarnAssets(f *ListFilter, assets []adapters.EarnAmount) bool {
	if f.Symbol == "" {
		return true
	}
	for _, asset := range assets {
		if matchesSymbol(f.Symbol, asset.PlatformAssetSymbol, optionalSymbol(asset.AssetSymbol)) {
			return true
		}
	}
	return false
}

func (s *Service) FlatEarnPositions(ctx context.Context, p PortfolioContext, f *ListFilter) (EarnList, error) {
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
		BalanceSummaries: []EarnBalanceSummary{},
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
		rows, summaries, collected, err := s.readEarnReport(ctx, p, account)
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
		for _, summary := range summaries {
			if !matchesEarnAssets(f, summary.Assets) {
				continue
			}
			if f.Symbol != "" {
				summary.Assets = slices.DeleteFunc(summary.Assets, func(asset adapters.EarnAmount) bool {
					return !matchesSymbol(f.Symbol, asset.PlatformAssetSymbol, optionalSymbol(asset.AssetSymbol))
				})
			}
			out.BalanceSummaries = append(out.BalanceSummaries, summary)
			out.Issues = appendNonStaleIssues(out.Issues, summary.Issues)
		}
	}
	sortEarnPositions(all, accountRanks(accounts))
	out.paginate(all)
	return out, nil
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
