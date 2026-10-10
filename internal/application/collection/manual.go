package collection

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func (s *Service) collectManual(ctx context.Context, portfolioID, jobID uuid.UUID, accounts []manual.CycleAccount, reported map[uuid.UUID]bool) error {
	canonicalAssetSymbols := map[adapters.Platform][]string{}
	for _, account := range accounts {
		if reported[account.AccountID] {
			continue
		}
		for _, balance := range account.Items {
			if !adapters.USDEquivalent(balance.CanonicalAssetSymbol) {
				canonicalAssetSymbols[account.Platform] = append(canonicalAssetSymbols[account.Platform], balance.CanonicalAssetSymbol)
			}
		}
	}
	prices := map[adapters.Platform][]adapters.Price{}
	for _, platform := range adapters.OfferedPlatforms() {
		if len(canonicalAssetSymbols[platform]) == 0 {
			continue
		}
		adapter, ok := s.adapters.Lookup(platform)
		if !ok {
			continue
		}
		market, ok := adapter.(adapters.MarketData)
		if !ok {
			continue
		}
		batch, err := market.CollectPrices(ctx, canonicalAssetSymbols[platform])
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Public unavailability retains quantities; valuation names prices it cannot resolve.
		if err == nil {
			prices[platform] = batch
		}
	}
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		for _, platform := range adapters.OfferedPlatforms() {
			for _, price := range prices[platform] {
				if err := s.catalog.RecordPrice(ctx, q, platform, price); err != nil && !errors.Is(err, catalog.ErrInvalid) {
					return err
				}
			}
		}
		for _, account := range accounts {
			if reported[account.AccountID] {
				continue
			}
			at := s.now().UTC()
			report := adapters.AccountReport{AccountType: adapters.TypeManual, Status: adapters.StatusOK, ObservedAt: at}
			for _, balance := range account.Items {
				report.Balances = append(report.Balances, adapters.Balance{AssetSymbol: balance.AssetSymbol, Equity: balance.Quantity})
			}
			if err := s.insertSuccessfulReport(ctx, q, portfolioID, jobID, account.Platform, account.AccountID, &report, "", at, at); err != nil {
				return err
			}
			state, err := manual.Quantities(account.Items)
			if err != nil {
				return err
			}
			if err := q.SetManualReportMetadata(ctx, sqlcgen.SetManualReportMetadataParams{PortfolioID: portfolioID[:], JobID: jobID[:], AccountID: account.AccountID[:], StateJson: state, ModifiedAt: account.ModifiedAt}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, account := range accounts {
		reported[account.AccountID] = true
	}
	return nil
}
