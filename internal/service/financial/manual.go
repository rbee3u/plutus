package financial

import (
	"bytes"
	"context"
	"slices"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/ordering"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Current quantities affect presentation only; report amounts and totals stay frozen.
func (s *Service) currentManualBalances(ctx context.Context, p PortfolioContext) (map[uuid.UUID][]BalanceRow, error) {
	rows, _, err := s.manualOverlayBalances(ctx, p, nil)
	return rows, err
}

func (s *Service) manualOverlayBalances(ctx context.Context, p PortfolioContext, selected map[uuid.UUID]bool) (map[uuid.UUID][]BalanceRow, *ManualOverlay, error) {
	q := mysql.Queries(ctx, s.db)
	var records []sqlcgen.ListPortfolioManualRevisionsRow
	if selected == nil {
		var err error
		records, err = q.ListPortfolioManualRevisions(ctx, p.ID[:])
		if err != nil {
			return nil, nil, err
		}
	} else {
		ids := make([][]byte, 0, len(selected))
		for id, include := range selected {
			if include {
				ids = append(ids, id[:])
			}
		}
		for start := 0; start < len(ids); start += 500 {
			rows, err := q.ListManualRevisionsByAccounts(ctx, sqlcgen.ListManualRevisionsByAccountsParams{PortfolioID: p.ID[:], AccountIds: ids[start:min(start+500, len(ids))]})
			if err != nil {
				return nil, nil, err
			}
			for _, row := range rows {
				records = append(records, sqlcgen.ListPortfolioManualRevisionsRow(row))
			}
		}
		// Batches follow map iteration order; overlay versions retain account order.
		slices.SortFunc(records, func(a, b sqlcgen.ListPortfolioManualRevisionsRow) int {
			return bytes.Compare(a.AccountID, b.AccountID)
		})
	}
	out := map[uuid.UUID][]BalanceRow{}
	overlay := &ManualOverlay{Versions: []ManualLedgerVersion{}}
	ids := [][]byte{}
	assets := map[uuid.UUID]bool{}
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
			assets[assetID] = true
			quantity := decimal.Compact(item.Quantity)
			modified := item.ChangedAt
			out[accountID] = append(out[accountID], BalanceRow{AccountID: accountID, AssetID: assetID, AssetSymbol: item.AssetSymbol, CanonicalAssetSymbol: item.CanonicalAssetSymbol, Equity: quantity, ManualQuantity: &quantity, ManualModifiedAt: &modified, ManualChange: "added"})
		}
	}
	assetIDs := make([][]byte, 0, len(assets))
	for id := range assets {
		assetIDs = append(assetIDs, id[:])
	}
	excluded := map[uuid.UUID]bool{}
	for start := 0; start < len(assetIDs); start += 500 {
		rules, err := q.ListCurrentAssetExclusionsByAssets(ctx, sqlcgen.ListCurrentAssetExclusionsByAssetsParams{PortfolioID: p.ID[:], AssetIds: assetIDs[start:min(start+500, len(assetIDs))]})
		if err != nil {
			return nil, nil, err
		}
		for _, rule := range rules {
			id, err := uuid.FromBytes(rule.AssetID)
			if err != nil {
				return nil, nil, err
			}
			excluded[id] = rule.Excluded
		}
	}
	for _, rows := range out {
		for i := range rows {
			rows[i].Excluded = excluded[rows[i].AssetID]
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
		if row.AccountType != string(adapter.TypeManual) {
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
			AccountID: id, AccountName: adapter.AccountName(adapter.AccountType(row.AccountType)), AccountType: row.AccountType, Selected: true,
			IdentityID: identityID, IdentityName: row.IdentityName, IdentityStatus: "enabled",
			ConnectionStatus: row.ConnectionStatus, Platform: row.Platform, AccountStructure: row.AccountStructure,
			Status: "failed", Method: valuation.MethodMissing, Freshness: valuation.FreshnessStale,
		})
	}
	key := func(a valuation.AccountValuation) ordering.Account {
		return ordering.Account{
			Identity:  ordering.Identity{Platform: a.Platform, Name: a.IdentityName, ID: a.IdentityID},
			Structure: adapter.AccountStructure(a.AccountStructure), Type: adapter.AccountType(a.AccountType), ID: a.AccountID,
		}
	}
	slices.SortFunc(accounts, func(a, b valuation.AccountValuation) int { return ordering.CompareAccounts(key(a), key(b)) })
	return accounts, nil
}
