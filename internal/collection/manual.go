package collection

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/manual"
)

func (s *Service) collectManual(ctx context.Context, portfolioID, jobID uuid.UUID, accounts []manual.CycleAccount, reported map[uuid.UUID]bool) error {
	symbols := map[adapters.Platform][]string{}
	for _, account := range accounts {
		if reported[account.AccountID] {
			continue
		}
		for _, balance := range account.Items {
			if !adapters.USDEquivalent(balance.AssetSymbol) {
				symbols[account.Platform] = append(symbols[account.Platform], balance.AssetSymbol)
			}
		}
	}
	prices := map[adapters.Platform][]adapters.Price{}
	for _, platform := range adapters.OfferedPlatforms() {
		if len(symbols[platform]) == 0 {
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
		batch, err := market.CollectPrices(ctx, symbols[platform])
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Public unavailability retains quantities; valuation names prices it cannot resolve.
		if err == nil {
			prices[platform] = batch
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
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
			report.Balances = append(report.Balances, adapters.Balance{Symbol: balance.Symbol, Equity: balance.Quantity})
		}
		if err := s.insertOkReport(ctx, q, portfolioID, jobID, account.Platform, account.AccountID, &report, "", at, at); err != nil {
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
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, account := range accounts {
		reported[account.AccountID] = true
	}
	return nil
}
