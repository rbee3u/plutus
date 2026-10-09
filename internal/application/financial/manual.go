package financial

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/ordering"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Current quantities affect presentation only; report amounts and totals stay frozen.
func (s *Service) currentManualBalances(ctx context.Context, p PortfolioContext) (map[uuid.UUID][]BalanceRow, error) {
	records, err := s.q.ListPortfolioManualBalances(ctx, p.ID[:])
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]BalanceRow)
	for _, record := range records {
		accountID, err := uuid.FromBytes(record.AccountID)
		if err != nil {
			return nil, err
		}
		platformAssetID, err := uuid.FromBytes(record.PlatformAssetID)
		if err != nil {
			return nil, err
		}
		quantity := decimal.Compact(record.Quantity)
		out[accountID] = append(out[accountID], BalanceRow{
			AccountID: accountID, PlatformAssetID: platformAssetID, PlatformAssetSymbol: record.PlatformAssetSymbol,
			AssetSymbol: record.AssetSymbol, Equity: quantity, ManualQuantity: &quantity,
			ManualModifiedAt: &record.ModifiedAt, ManualChange: "added", Excluded: record.Excluded,
			Freshness: valuation.FreshnessFresh,
		})
	}
	return out, nil
}

func mergeManualBalances(collected, current []BalanceRow, account *valuation.AccountValuation) []BalanceRow {
	bySymbol := make(map[string]BalanceRow, len(current))
	for _, row := range current {
		bySymbol[row.PlatformAssetSymbol] = row
	}
	for i := range collected {
		row := &collected[i]
		record, exists := bySymbol[row.PlatformAssetSymbol]
		if !exists {
			row.ManualChange = "deleted"
			continue
		}
		delete(bySymbol, row.PlatformAssetSymbol)
		row.ManualQuantity = record.ManualQuantity
		if decimal.Compact(row.Equity) != record.Equity {
			row.ManualChange = "updated"
		}
	}
	for _, row := range current {
		if _, exists := bySymbol[row.PlatformAssetSymbol]; !exists {
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
	boundary, err := s.q.ListBoundaryAccounts(ctx, p.ID[:])
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
			AccountID: id, AccountName: row.DisplayName, AccountType: row.AccountType, Selected: true,
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
