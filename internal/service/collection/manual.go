package collection

import (
	"context"
	"database/sql"
	"slices"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/identity"
)

// collectManual writes every boundary manual account's report and the cycle's
// marker in one transaction, so a retry never captures manual accounts added
// after the reports committed.
func (s *Service) collectManual(ctx context.Context, portfolioID, cycleID uuid.UUID, reported map[uuid.UUID]bool, recorded recordedPrices) error {
	accounts, err := s.manual.CycleInputs(ctx, portfolioID)
	if err != nil {
		return err
	}
	canonicalAssetSymbols := map[adapter.Platform][]string{}
	for _, account := range accounts {
		for _, balance := range account.Items {
			if !adapter.USDEquivalent(balance.CanonicalAssetSymbol) {
				canonicalAssetSymbols[account.Platform] = append(canonicalAssetSymbols[account.Platform], balance.CanonicalAssetSymbol)
			}
		}
	}
	prices := map[adapter.Platform][]adapter.Price{}
	for _, platform := range adapter.OfferedPlatforms() {
		if len(canonicalAssetSymbols[platform]) == 0 {
			continue
		}
		platformAdapter, ok := s.adapters.Lookup(platform)
		if !ok {
			continue
		}
		market, ok := platformAdapter.(adapter.MarketData)
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
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return err
		}
		marked, err := q.LockCycleManualReportedAt(ctx, sqlcgen.LockCycleManualReportedAtParams{PortfolioID: portfolioID[:], ID: cycleID[:]})
		if err != nil || marked.Valid {
			return err
		}
		for _, platform := range adapter.OfferedPlatforms() {
			if err := s.recordPrices(ctx, q, portfolioID, platform, prices[platform], recorded); err != nil {
				return err
			}
		}
		for _, account := range accounts {
			at := s.now().UTC()
			report := adapter.AccountReport{AccountType: adapter.TypeManual, Status: adapter.StatusSucceeded, CollectedAt: at}
			for _, balance := range account.Items {
				report.Balances = append(report.Balances, adapter.Balance{AssetSymbol: balance.AssetSymbol, Equity: balance.Quantity})
			}
			revision := sql.NullInt64{Int64: int64(account.RevisionNumber), Valid: true} // #nosec G115 -- revision numbers are positive BIGINT UNSIGNED values far below 2^63.
			if err := s.insertSuccessfulReport(ctx, q, portfolioID, cycleID, account.Platform, account.AccountID, &report, "", at, at, revision); err != nil {
				return err
			}
		}
		return q.MarkCycleManualReported(ctx, sqlcgen.MarkCycleManualReportedParams{PortfolioID: portfolioID[:], ID: cycleID[:], At: sql.NullTime{Time: s.now().UTC(), Valid: true}})
	}); err != nil {
		return err
	}
	for _, account := range accounts {
		reported[account.AccountID] = true
	}
	return nil
}

// supplementPrices fetches mapped canonical prices before the report cutoff.
// The private report retains its original observation time and financial facts.
func (s *Service) supplementPrices(ctx context.Context, portfolioID uuid.UUID, item *identity.Collectable, batch *adapter.Batch) error {
	symbols := []string{}
	for _, report := range batch.Reports {
		if report.Status != adapter.StatusSucceeded {
			continue
		}
		for _, balance := range report.Balances {
			symbols = append(symbols, balance.AssetSymbol)
		}
	}
	if len(symbols) == 0 {
		return nil
	}
	rows, err := s.q.ListMappingsForSymbols(ctx, sqlcgen.ListMappingsForSymbolsParams{PortfolioID: portfolioID[:], Platform: string(item.Identity.Platform), Symbols: symbols})
	if err != nil {
		return err
	}
	requested := map[string]bool{}
	for _, row := range rows {
		if row.CanonicalAssetSymbol != row.Symbol && !adapter.USDEquivalent(row.CanonicalAssetSymbol) {
			requested[row.CanonicalAssetSymbol] = true
		}
	}
	symbols = symbols[:0]
	for symbol := range requested {
		symbols = append(symbols, symbol)
	}
	slices.Sort(symbols)
	platformAdapter, ok := s.adapters.Lookup(item.Identity.Platform)
	if !ok || len(symbols) == 0 {
		return nil
	}
	market, ok := platformAdapter.(adapter.MarketData)
	if !ok {
		return nil
	}
	prices, err := market.CollectPrices(ctx, symbols)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil {
		batch.Prices = append(batch.Prices, prices...)
	}
	return nil
}
