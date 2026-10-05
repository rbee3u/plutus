package valuation

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
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
	Issues           []Issue
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
	Issues           []Issue
}

type EarnList struct {
	FlatList[EarnPositionRow]
	BalanceSummaries []EarnBalanceSummary
	DetailsComplete  bool
}

func (s *Service) readEarnReport(ctx context.Context, p PortfolioContext, account *AccountValuation) ([]EarnPositionRow, []EarnBalanceSummary, bool, error) {
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
	assets := map[string]string{}
	for _, b := range balances {
		assets[b.Symbol] = b.AssetSymbol
	}
	if !collected {
		issue := Issue{Code: "earn_details_unavailable", AccountID: &account.AccountID, AccountName: account.AccountName, Detail: "Earn product details were not collected in this Account Report. Known Balance amounts are shown separately."}
		summary := EarnBalanceSummary{AccountID: account.AccountID, AccountName: account.AccountName, IdentityName: account.IdentityName, Platform: account.Platform, Freshness: account.Freshness, ReportObservedAt: account.ReportObservedAt, Assets: []adapters.EarnAmount{}, Issues: []Issue{issue}}
		for _, b := range balances {
			summary.Assets = append(summary.Assets, adapters.EarnAmount{Symbol: b.Symbol, Amount: b.Equity, AssetSymbol: adapters.DualText(b.AssetSymbol)})
		}
		if account.Freshness == FreshnessStale {
			summary.Issues = append(summary.Issues, earnStaleIssue(account))
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
			position.Assets[i].AssetSymbol = adapters.DualText(assets[position.Assets[i].Symbol])
		}
		slices.SortFunc(position.Assets, func(a, b adapters.EarnAmount) int { return strings.Compare(earnAssetKey(a), earnAssetKey(b)) })
		row := EarnPositionRow{EarnPosition: position, AccountID: account.AccountID, AccountName: account.AccountName, AccountType: account.AccountType, IdentityID: account.IdentityID, IdentityName: account.IdentityName, Platform: account.Platform, Freshness: account.Freshness, ReportObservedAt: account.ReportObservedAt, Issues: []Issue{}}
		for _, problem := range position.Problems {
			row.Issues = append(row.Issues, Issue{Code: "earn_metadata_unavailable", AccountID: &row.AccountID, AccountName: row.AccountName, Detail: "Earn position " + row.PlatformPositionRef + ": " + problem})
		}
		if account.Freshness == FreshnessStale {
			row.Issues = append(row.Issues, earnStaleIssue(account))
		}
		items = append(items, row)
	}
	return items, summaries, true, nil
}

func earnStaleIssue(account *AccountValuation) Issue {
	for _, issue := range accountIssues(account) {
		if issue.Code == "stale" {
			return issue
		}
	}
	return Issue{Code: "stale", AccountID: &account.AccountID, AccountName: account.AccountName, Detail: "An earlier successful Account Report is carried forward."}
}

func earnAssetKey(a adapters.EarnAmount) string {
	if a.AssetSymbol != nil {
		return *a.AssetSymbol + "\x00" + a.Symbol
	}
	return a.Symbol + "\x00" + a.Symbol
}

func sortEarnPositions(rows []EarnPositionRow, ranks map[uuid.UUID]int) {
	types := map[string]int{"flexible": 0, "fixed": 1, "bonded": 2, "timed": 3, "on_chain": 4}
	slices.SortFunc(rows, func(a, b EarnPositionRow) int {
		if c := cmp.Compare(ranks[a.AccountID], ranks[b.AccountID]); c != 0 {
			return c
		}
		if c := slices.CompareFunc(a.Assets, b.Assets, func(a, b adapters.EarnAmount) int { return strings.Compare(earnAssetKey(a), earnAssetKey(b)) }); c != 0 {
			return c
		}
		if c := cmp.Compare(types[a.Type], types[b.Type]); c != 0 {
			return c
		}
		if c := compareExpiry(a.MaturityAt, b.MaturityAt); c != 0 {
			return c
		}
		return strings.Compare(a.PlatformPositionRef, b.PlatformPositionRef)
	})
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
	for _, r := range view.EarnPositions {
		for _, i := range r.Issues {
			if i.Code != "stale" {
				view.Issues = append(view.Issues, i)
			}
		}
	}
	for _, r := range view.EarnBalanceSummaries {
		for _, i := range r.Issues {
			if i.Code != "stale" {
				view.Issues = append(view.Issues, i)
			}
		}
	}
	return view, nil
}

func matchesEarnAssets(f *ListFilter, assets []adapters.EarnAmount) bool {
	if f.Symbol == "" {
		return true
	}
	for _, a := range assets {
		if matchesSymbol(f.Symbol, a.Symbol, optionalSymbol(a.AssetSymbol)) {
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
	out := EarnList{FlatList: FlatList[EarnPositionRow]{Context: c, Items: []EarnPositionRow{}, Offset: offset, Limit: limit, Completeness: CompletenessIncomplete, Issues: []Issue{}}, BalanceSummaries: []EarnBalanceSummary{}}
	if point == nil {
		out.Issues = append(out.Issues, Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
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
		for _, r := range rows {
			if matchesEarnAssets(f, r.Assets) {
				all = append(all, r)
				for _, issue := range r.Issues {
					if issue.Code != "stale" {
						out.Issues = append(out.Issues, issue)
					}
				}
			}
		}
		for _, r := range summaries {
			if !matchesEarnAssets(f, r.Assets) {
				continue
			}
			if f.Symbol != "" {
				r.Assets = slices.DeleteFunc(r.Assets, func(a adapters.EarnAmount) bool { return !matchesEarnAssets(f, []adapters.EarnAmount{a}) })
			}
			out.BalanceSummaries = append(out.BalanceSummaries, r)
			for _, issue := range r.Issues {
				if issue.Code != "stale" {
					out.Issues = append(out.Issues, issue)
				}
			}
		}
	}
	sortEarnPositions(all, accountRanks(accounts))
	out.Total = len(all)
	if offset < len(all) {
		out.Items = all[offset:min(offset+limit, len(all))]
	}
	return out, nil
}
