package financial

import (
	"context"
	"slices"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/ordering"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Current quantities affect presentation only; report amounts and totals stay frozen.
func (s *Service) currentManualBalances(ctx context.Context, p PortfolioContext) (map[uuid.UUID][]BalanceRow, error) {
	records, err := mysql.Queries(ctx, s.db).ListPortfolioManualRevisions(ctx, p.ID[:])
	if err != nil {
		return nil, err
	}
	rules, err := mysql.Queries(ctx, s.db).ListCurrentAssetExclusions(ctx, p.ID[:])
	if err != nil {
		return nil, err
	}
	excluded := map[uuid.UUID]bool{}
	for _, rule := range rules {
		id, err := uuid.FromBytes(rule.AssetID)
		if err != nil {
			return nil, err
		}
		excluded[id] = rule.Excluded
	}
	out := map[uuid.UUID][]BalanceRow{}
	for _, record := range records {
		accountID, err := uuid.FromBytes(record.AccountID)
		if err != nil {
			return nil, err
		}
		items, err := manual.Hydrate(ctx, mysql.Queries(ctx, s.db), p.ID[:], record.AccountID, record.RevisionNumber)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			quantity := decimal.Compact(item.Quantity)
			out[accountID] = append(out[accountID], BalanceRow{AccountID: accountID, AssetID: item.AssetID, AssetSymbol: item.AssetSymbol, CanonicalAssetSymbol: item.CanonicalAssetSymbol, Equity: quantity, ManualQuantity: &quantity, ManualModifiedAt: &item.ModifiedAt, ManualChange: "added", Excluded: excluded[item.AssetID], Freshness: valuation.FreshnessFresh})
		}
	}
	return out, nil
}

func mergeManualBalances(collected, current []BalanceRow, account *valuation.AccountValuation) []BalanceRow {
	bySymbol := make(map[string]BalanceRow, len(current))
	for _, row := range current {
		bySymbol[row.AssetSymbol] = row
	}
	for i := range collected {
		row := &collected[i]
		record, exists := bySymbol[row.AssetSymbol]
		if !exists {
			row.ManualChange = "deleted"
			continue
		}
		delete(bySymbol, row.AssetSymbol)
		row.ManualQuantity = record.ManualQuantity
		if decimal.Compact(row.Equity) != record.Equity {
			row.ManualChange = "updated"
		}
	}
	for _, row := range current {
		if _, exists := bySymbol[row.AssetSymbol]; !exists {
			continue
		}
		row.AccountName, row.AccountType = account.AccountName, account.AccountType
		row.IdentityID, row.IdentityName, row.Platform = account.IdentityID, account.IdentityName, account.Platform
		collected = append(collected, row)
	}
	return collected
}

// Include Manual Accounts before their first report so current records remain reachable.
func (s *Service) withManualAccounts(ctx context.Context, p PortfolioContext, accounts []valuation.AccountValuation) ([]valuation.AccountValuation, error) {
	boundary, err := mysql.Queries(ctx, s.db).ListBoundaryAccounts(ctx, p.ID[:])
	if err != nil {
		return nil, err
	}
	seen := indexAccounts(accounts)
	for _, row := range boundary {
		if row.AccountType != string(adapters.TypeManual) {
			continue
		}
		id, err := uuid.FromBytes(row.ID)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		identityID, err := uuid.FromBytes(row.IdentityID)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, valuation.AccountValuation{
			AccountID: id, AccountName: adapters.AccountName(adapters.AccountType(row.AccountType)), AccountType: row.AccountType, Selected: true,
			IdentityID: identityID, IdentityName: row.IdentityName, IdentityStatus: "enabled",
			ConnectionStatus: row.ConnectionStatus, Platform: row.Platform, AccountStructure: row.AccountStructure,
			Status: "failed", Method: valuation.MethodMissing, Freshness: valuation.FreshnessStale,
		})
	}
	key := func(a valuation.AccountValuation) ordering.Account {
		return ordering.Account{
			Identity:  ordering.Identity{Platform: a.Platform, Name: a.IdentityName, ID: a.IdentityID},
			Structure: adapters.AccountStructure(a.AccountStructure), Type: adapters.AccountType(a.AccountType), ID: a.AccountID,
		}
	}
	slices.SortFunc(accounts, func(a, b valuation.AccountValuation) int { return ordering.CompareAccounts(key(a), key(b)) })
	return accounts, nil
}
