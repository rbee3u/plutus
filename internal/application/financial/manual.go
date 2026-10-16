package financial

import (
	"context"
	"slices"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/ordering"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Current quantities affect presentation only; report amounts and totals stay frozen.
func (s *Service) currentManualBalances(ctx context.Context, p PortfolioContext) (map[uuid.UUID][]BalanceRow, error) {
	rows, _, err := s.manualOverlayBalances(ctx, p, nil)
	return rows, err
}

func (s *Service) manualOverlayBalances(ctx context.Context, p PortfolioContext, selected map[uuid.UUID]bool) (map[uuid.UUID][]BalanceRow, *ManualOverlay, error) {
	records, err := mysql.Queries(ctx, s.db).ListPortfolioManualRevisions(ctx, p.ID[:])
	if err != nil {
		return nil, nil, err
	}
	rules, err := mysql.Queries(ctx, s.db).ListCurrentAssetExclusions(ctx, p.ID[:])
	if err != nil {
		return nil, nil, err
	}
	excluded := map[uuid.UUID]bool{}
	for _, rule := range rules {
		id, err := uuid.FromBytes(rule.AssetID)
		if err != nil {
			return nil, nil, err
		}
		excluded[id] = rule.Excluded
	}
	out := map[uuid.UUID][]BalanceRow{}
	overlay := &ManualOverlay{Versions: []ManualLedgerVersion{}}
	ids := [][]byte{}
	for _, record := range records {
		accountID, err := uuid.FromBytes(record.AccountID)
		if err != nil {
			return nil, nil, err
		}
		if selected != nil && !selected[accountID] {
			continue
		}
		ids = append(ids, record.AccountID)
		overlay.Versions = append(overlay.Versions, ManualLedgerVersion{AccountID: accountID, Version: record.RevisionNumber})
	}
	for start := 0; start < len(ids); start += 500 {
		items, err := mysql.Queries(ctx, s.db).ListCurrentManualBalancesByAccounts(ctx, sqlcgen.ListCurrentManualBalancesByAccountsParams{PortfolioID: p.ID[:], AccountIds: ids[start:min(start+500, len(ids))]})
		if err != nil {
			return nil, nil, err
		}
		for _, item := range items {
			accountID, err := uuid.FromBytes(item.AccountID)
			if err != nil {
				return nil, nil, err
			}
			assetID, err := uuid.FromBytes(item.AssetID)
			if err != nil {
				return nil, nil, err
			}
			quantity := decimal.Compact(item.Quantity)
			modified := item.ChangedAt
			out[accountID] = append(out[accountID], BalanceRow{AccountID: accountID, AssetID: assetID, AssetSymbol: item.AssetSymbol, CanonicalAssetSymbol: item.CanonicalAssetSymbol, Equity: quantity, ManualQuantity: &quantity, ManualModifiedAt: &modified, ManualChange: "added", Excluded: excluded[assetID]})
		}
	}
	return out, overlay, nil
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

// Include manual accounts before their first report so current records remain reachable.
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
