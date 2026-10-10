package valuation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// prepareReport resolves a compatible source and only the prices its method needs.
// A nil preload selects ordinary SQL reads; rebuild supplies its bounded batch.
func (s *Service) prepareReport(ctx context.Context, book priceResolver, row *sqlcgen.ListReportsByJobRow, ignored map[uuid.UUID]bool, inputs *reportInputs) (preparedReport, error) {
	item, err := accountFromRow(row)
	if err != nil {
		return preparedReport{}, err
	}
	item.Omitted = nil
	item.ValueUSD = nil
	item.valueRat = nil
	item.SourceReportID = nil
	var sourceID uuid.UUID
	var observedAt time.Time
	equity := row.Equity
	if row.Status == "ok" {
		sourceID = item.ReportID
		observedAt = row.ObservedAt.Time
		item.Freshness = FreshnessFresh
		item.SourceMarginMode = nullStringPtr(row.MarginMode)
	} else {
		var src sqlcgen.GetLatestOkReportForAccountRow
		if inputs == nil {
			src, err = s.q.GetLatestOkReportForAccount(ctx, sqlcgen.GetLatestOkReportForAccountParams{PortfolioID: row.PortfolioID, AccountID: row.AccountID, ScopeVersion: row.ScopeVersion, Before: row.CreatedAt})
		} else if previous := inputs.sources[item.ReportID]; previous != nil {
			src = *previous
		} else {
			err = sql.ErrNoRows
		}
		if errors.Is(err, sql.ErrNoRows) {
			item.Method = MethodMissing
			item.Freshness = FreshnessStale
			item.Equity = nil
			item.AdjustedEquity = nil
			item.AvailableEquity = nil
			item.InitialMarginRatio = nil
			item.MaintenanceMarginRatio = nil
			item.UnifiedMarginRatio = nil
			item.ReportObservedAt = nil
			return preparedReport{account: item}, nil
		}
		if err != nil {
			return preparedReport{}, err
		}
		sourceID, err = uuid.FromBytes(src.ID)
		if err != nil {
			return preparedReport{}, err
		}
		observedAt = src.ObservedAt.Time
		item.SourceMarginMode = nullStringPtr(src.MarginMode)
		equity = src.Equity
		item.Equity = nullDecimal(src.Equity)
		item.AdjustedEquity = nullDecimal(src.AdjustedEquity)
		item.AvailableEquity = nullDecimal(src.AvailableEquity)
		item.InitialMarginRatio = nullDecimal(src.InitialMarginRatio)
		item.MaintenanceMarginRatio = nullDecimal(src.MaintenanceMarginRatio)
		item.UnifiedMarginRatio = nullDecimal(src.UnifiedMarginRatio)
		item.Freshness = FreshnessStale
	}
	item.SourceReportID = &sourceID
	item.ReportObservedAt = &observedAt
	if equity.Valid {
		if value, ok := decimal.Parse(equity.String); ok {
			return preparedReport{account: item, reported: value}, nil
		}
	}
	var balances []sqlcgen.ListBalancesByReportRow
	if inputs == nil {
		balances, err = s.q.ListBalancesByReport(ctx, sourceID[:])
		if err != nil {
			return preparedReport{}, err
		}
	} else {
		var loaded bool
		balances, loaded = inputs.balances[sourceID]
		if !loaded {
			return preparedReport{}, fmt.Errorf("balances were not loaded for source report %s", sourceID)
		}
	}
	input := preparedReport{account: item, balances: make([]preparedBalance, 0, len(balances))}
	for i := range balances {
		balance := &balances[i]
		assetID, err := uuid.FromBytes(balance.AssetID)
		if err != nil {
			return preparedReport{}, err
		}
		if ignored[assetID] {
			continue
		}
		row := preparedBalance{assetSymbol: balance.AssetSymbol, canonicalAssetSymbol: balance.CanonicalAssetSymbol, rawEquity: balance.Equity}
		equity, ok := decimal.Parse(balance.Equity)
		if !ok {
			input.balances = append(input.balances, row)
			continue
		}
		if equity.Sign() == 0 {
			continue
		}
		row.equity = equity
		lookup := book.Lookup
		if item.AccountType == string(adapters.TypeManual) {
			lookup = book.LookupPlatform
		}
		price, found, err := lookup(ctx, balance.Platform, balance.CanonicalAssetSymbol, observedAt)
		if err != nil {
			return preparedReport{}, err
		}
		if found {
			row.price = price
		}
		input.balances = append(input.balances, row)
	}
	return input, nil
}

// previousAccounts returns the Account values of the point immediately before evaluatedAt.
func (s *Service) previousAccounts(ctx context.Context, portfolioID uuid.UUID, evaluatedAt time.Time, jobID uuid.UUID) (map[uuid.UUID]previousAccount, error) {
	prev, err := s.q.GetValuationPointBefore(ctx, sqlcgen.GetValuationPointBeforeParams{PortfolioID: portfolioID[:], At: evaluatedAt})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // a nil map means "no previous point", which disables Boundary Event derivation.
	}
	if err != nil {
		return nil, err
	}
	prevJob, err := uuid.FromBytes(prev.JobID)
	if err != nil {
		return nil, err
	}
	if prevJob == jobID {
		return nil, nil //nolint:nilnil // rebuilding the first point: nothing precedes it.
	}
	rows, err := s.q.ListReportsByJob(ctx, sqlcgen.ListReportsByJobParams{PortfolioID: portfolioID[:], JobID: prev.JobID})
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]previousAccount, len(rows))
	for i := range rows {
		accountID, err := uuid.FromBytes(rows[i].AccountID)
		if err != nil {
			return nil, err
		}
		identityID, err := uuid.FromBytes(rows[i].IdentityID)
		if err != nil {
			return nil, err
		}
		entry := previousAccount{manualState: canonicalManualState(rows[i].ManualStateJson), scopeVersion: rows[i].ScopeVersion, accountType: rows[i].AccountType, name: rows[i].AccountName, identityID: identityID, identityName: rows[i].IdentityName, platform: rows[i].Platform, method: rows[i].Method}
		if rows[i].ValueUsd.Valid {
			if value, ok := decimal.Parse(rows[i].ValueUsd.String); ok {
				entry.value = value
			}
		}
		out[accountID] = entry
	}
	return out, nil
}
