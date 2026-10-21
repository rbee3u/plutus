package valuation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type reportInputs struct {
	sources  map[uuid.UUID]*sqlcgen.AccountReport
	balances map[uuid.UUID][]sqlcgen.ListBalancesByReportRow
}

// prepareReport freezes every balance decision, even descriptive reported totals.
func (s *Service) prepareReport(ctx context.Context, book priceResolver, row *sqlcgen.ListReportsByCycleRow, excluded map[uuid.UUID]bool, inputs *reportInputs) (preparedReport, error) {
	item, err := reportFromRow(row)
	if err != nil {
		return preparedReport{}, err
	}
	item.Omitted, item.ValueUSD, item.valueRat, item.SourceReportID = nil, nil, nil, nil
	sourceID := item.ReportID
	observedAt, cutoff := effectiveObservedAt(row.CollectedAt, row.SourceObservedAt), row.RecordedAt
	equity, adjustment := row.ReportedEquityUsd, row.SpotMarginUnrealizedPnlUsd
	if row.Status == adapter.StatusSucceeded {
		item.Freshness = FreshnessFresh
	} else {
		var src sqlcgen.AccountReport
		if inputs == nil {
			src, err = mysql.Queries(ctx, s.db).GetLatestSucceededReportForAccount(ctx, sqlcgen.GetLatestSucceededReportForAccountParams{PortfolioID: row.PortfolioID, AccountID: row.AccountID, Before: row.RecordedAt})
		} else if previous := inputs.sources[item.ReportID]; previous != nil {
			src = *previous
		} else {
			err = sql.ErrNoRows
		}
		if errors.Is(err, sql.ErrNoRows) {
			item.Method, item.Freshness = MethodMissing, FreshnessStale
			item.Equity, item.AdjustedEquity, item.AvailableEquity = nil, nil, nil
			item.InitialMarginRatio, item.MaintenanceMarginRatio, item.MarginRatio = nil, nil, nil
			item.ReportObservedAt, item.SourceMarginMode = nil, nil
			return preparedReport{account: item}, nil
		}
		if err != nil {
			return preparedReport{}, err
		}
		sourceID, err = uuid.FromBytes(src.ID)
		if err != nil {
			return preparedReport{}, err
		}
		observedAt, cutoff = effectiveObservedAt(src.CollectedAt, src.SourceObservedAt), src.RecordedAt
		equity, adjustment = src.ReportedEquityUsd, src.SpotMarginUnrealizedPnlUsd
		item.SourceMarginMode = nullStringPtr(src.MarginMode)
		item.Equity, item.AdjustedEquity, item.AvailableEquity = nullDecimal(src.ReportedEquityUsd), nullDecimal(src.AdjustedEquityUsd), nullDecimal(src.AvailableEquityUsd)
		item.InitialMarginRatio = MarginRatio(src.ReportedInitialMarginRatio, src.InitialMarginRequirementUsd, src.AdjustedEquityUsd)
		item.MaintenanceMarginRatio = MarginRatio(src.ReportedMaintenanceMarginRatio, src.MaintenanceMarginRequirementUsd, src.AdjustedEquityUsd)
		item.MarginRatio = nullDecimal(src.ReportedMarginRatio)
		item.Freshness = FreshnessStale
	}
	item.SourceReportID, item.ReportObservedAt = &sourceID, &observedAt
	input := preparedReport{account: item}
	if equity.Valid {
		input.reported, _ = decimal.Parse(equity.String)
	}
	if adjustment.Valid {
		input.adjustment, _ = decimal.Parse(adjustment.String)
	}
	var balances []sqlcgen.ListBalancesByReportRow
	if inputs == nil {
		balances, err = mysql.Queries(ctx, s.db).ListBalancesByReport(ctx, sqlcgen.ListBalancesByReportParams{PortfolioID: row.PortfolioID, ReportID: sourceID[:]})
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
	if live, ok := book.(*PriceBook); ok {
		symbols := make([]string, 0, len(balances))
		for _, balance := range balances {
			symbols = append(symbols, balance.CanonicalAssetSymbol)
		}
		if err := live.Preload(ctx, item.Platform, symbols, cutoff); err != nil {
			return preparedReport{}, err
		}
	}
	for _, balance := range balances {
		id, err := uuid.FromBytes(balance.AssetID)
		if err != nil {
			return preparedReport{}, err
		}
		canonical := balance.CanonicalAssetSymbol
		decision := BalanceValuation{AssetID: id, CanonicalAssetSymbol: canonical, Treatment: "included", PricingMethod: "unavailable"}
		if input.reported != nil {
			decision.Treatment = "descriptive"
		}
		if excluded[id] && input.reported == nil {
			decision.Treatment, decision.PricingMethod = "excluded", "excluded"
		}
		prepared := preparedBalance{assetSymbol: balance.AssetSymbol, canonicalAssetSymbol: canonical, rawEquity: balance.Equity, decision: decision}
		prepared.equity, _ = decimal.Parse(balance.Equity)
		if prepared.decision.Treatment != "excluded" && prepared.equity != nil {
			if prepared.equity.Sign() == 0 {
				prepared.price = big.NewRat(0, 1)
				prepared.decision.PricingMethod = "zero_equity"
			} else {
				price, err := book.Resolve(ctx, balance.Platform, canonical, cutoff)
				if err != nil {
					return preparedReport{}, err
				}
				prepared.price = price.Value
				prepared.decision.PricingMethod, prepared.decision.PriceObservationID = price.Method, price.ObservationID
			}
		}
		input.balances = append(input.balances, prepared)
	}
	return input, nil
}

// previousAccounts returns the account values of the point immediately before evaluatedAt.
func (s *Service) previousAccounts(ctx context.Context, portfolioID uuid.UUID, evaluatedAt time.Time, cycleID uuid.UUID) (map[uuid.UUID]previousAccount, error) {
	prev, err := mysql.Queries(ctx, s.db).GetValuationPointBefore(ctx, sqlcgen.GetValuationPointBeforeParams{PortfolioID: portfolioID[:], At: evaluatedAt, CycleID: cycleID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // a nil map means "no previous point", which disables boundary event derivation.
	}
	if err != nil {
		return nil, err
	}
	prevCycle, err := uuid.FromBytes(prev.CycleID)
	if err != nil {
		return nil, err
	}
	if prevCycle == cycleID {
		return nil, nil //nolint:nilnil // the first point has no predecessor.
	}
	rows, err := s.accountsByCycles(ctx, portfolioID, [][]byte{prev.CycleID})
	if err != nil {
		return nil, err
	}
	return previousFromAccounts(rows[prevCycle]), nil
}

func previousFromAccounts(accounts []AccountValuation) map[uuid.UUID]previousAccount {
	previous := make(map[uuid.UUID]previousAccount, len(accounts))
	for _, a := range accounts {
		entry := previousAccount{accountType: a.AccountType, name: a.AccountName, identityID: a.IdentityID, identityName: a.IdentityName, platform: a.Platform, method: a.Method, complete: a.Completeness() == CompletenessComplete}
		if a.ValueUSD != nil {
			entry.value, _ = decimal.Parse(*a.ValueUSD)
		}
		previous[a.AccountID] = entry
	}
	return previous
}
