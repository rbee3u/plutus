package collection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// recordedPrices holds the Platform and Canonical Asset pairs already priced in
// this cycle attempt. Any write error aborts the attempt, so the set never
// outlives a rolled-back transaction.
type recordedPrices map[adapters.Platform]map[string]bool

// recordPrices stores at most one observation per Platform and Canonical Asset
// per cycle attempt; later Identities on that Platform reuse it.
func (s *Service) recordPrices(ctx context.Context, q *sqlcgen.Queries, portfolioID uuid.UUID, platform adapters.Platform, prices []adapters.Price, recorded recordedPrices) error {
	for _, price := range prices {
		symbol, err := catalog.NormalizeSymbol(price.CanonicalAssetSymbol)
		if err != nil || price.Price == "" || recorded[platform][symbol] {
			continue
		}
		if err := s.catalog.RecordPrice(ctx, q, portfolioID, platform, price); err != nil {
			if errors.Is(err, catalog.ErrInvalid) {
				continue
			}
			return err
		}
		if recorded[platform] == nil {
			recorded[platform] = map[string]bool{}
		}
		recorded[platform][symbol] = true
	}
	return nil
}

func (s *Service) storeBatch(ctx context.Context, q *sqlcgen.Queries, portfolioID, cycleID uuid.UUID, item *identity.Collectable, batch *adapters.Batch, byType map[adapters.AccountType]identity.Account, written map[uuid.UUID]bool, failed *int, prices recordedPrices, now time.Time) error {
	if err := s.recordPrices(ctx, q, portfolioID, item.Identity.Platform, batch.Prices, prices); err != nil {
		return err
	}
	for i := range batch.Reports {
		report := &batch.Reports[i]
		account, ok := byType[report.AccountType]
		if !ok || written[account.ID] {
			continue
		}
		if report.Status == adapters.StatusSucceeded {
			for _, balance := range report.Balances {
				if !decimal.Valid(balance.Equity) {
					report.Status, report.ErrorClass, report.ErrorDetail = adapters.StatusFailed, adapters.ClassNormalization, "A required Balance equity is unavailable."
					break
				}
			}
		}
		if report.Status != adapters.StatusSucceeded {
			class := report.ErrorClass
			if class == "" {
				class = adapters.ClassTemporary
			}
			if err := insertFailedReport(ctx, q, portfolioID, cycleID, account.ID, class, report.ErrorDetail, now); err != nil {
				return err
			}
			written[account.ID] = true
			*failed++
			continue
		}
		if err := s.insertSuccessfulReport(ctx, q, portfolioID, cycleID, item.Identity.Platform, account.ID, report, batch.MarginMode, batch.CollectedAt, now, sql.NullInt64{}); err != nil {
			return err
		}
		written[account.ID] = true
	}
	return nil
}

func insertFailedReport(ctx context.Context, q *sqlcgen.Queries, portfolioID, cycleID, accountID uuid.UUID, class, detail string, now time.Time) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	if detail == "" {
		detail = "The platform call failed."
	}
	return q.InsertAccountReport(ctx, sqlcgen.InsertAccountReportParams{
		ID: id[:], PortfolioID: portfolioID[:], CycleID: cycleID[:], AccountID: accountID[:], Status: adapters.StatusFailed,
		ErrorClass: sql.NullString{String: class, Valid: true}, ErrorDetail: sql.NullString{String: truncate(detail, 1024), Valid: true},
		RecordedAt: now,
	})
}

// manualRevision is set only for Manual reports, which copy that revision.
func (s *Service) insertSuccessfulReport(ctx context.Context, q *sqlcgen.Queries, portfolioID, cycleID uuid.UUID, platform adapters.Platform, accountID uuid.UUID, report *adapters.AccountReport, mode adapters.MarginMode, batchObserved, now time.Time, manualRevision sql.NullInt64) error {
	if report.AccountType == "earn" || len(report.EarnPositions) > 0 {
		if report.AccountType != "earn" {
			return fmt.Errorf("%w: Earn details outside a collected Earn report", catalog.ErrInvalid)
		}
		if err := adapters.CheckEarnPositions(report.EarnPositions, report.Balances); err != nil {
			return err
		}
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	observed := report.CollectedAt
	if observed.IsZero() {
		observed = batchObserved
	}
	if observed.IsZero() {
		observed = now
	}
	if err := q.InsertAccountReport(ctx, sqlcgen.InsertAccountReportParams{
		ID: id[:], PortfolioID: portfolioID[:], CycleID: cycleID[:], AccountID: accountID[:], Status: adapters.StatusSucceeded,

		CollectedAt:                     sql.NullTime{Time: observed.UTC(), Valid: true},
		SourceObservedAt:                timeColumn(report.SourceObservedAt),
		MarginMode:                      nonEmptyStringColumn(string(mode)),
		InitialMarginRequirementUsd:     decimalColumn(report.InitialMarginRequirementUSD),
		MaintenanceMarginRequirementUsd: decimalColumn(report.MaintenanceMarginRequirementUSD),
		SpotMarginUnrealizedPnlUsd:      decimalColumn(report.SpotMarginUnrealizedPnlUSD),
		ReportedEquityUsd:               decimalColumn(report.Equity),
		AdjustedEquityUsd:               decimalColumn(report.AdjustedEquity),
		AvailableEquityUsd:              decimalColumn(report.AvailableEquity),
		ReportedInitialMarginRatio:      decimalColumn(report.ReportedInitialMarginRatio),
		ReportedMaintenanceMarginRatio:  decimalColumn(report.ReportedMaintenanceMarginRatio),
		ReportedMarginRatio:             decimalColumn(report.ReportedMarginRatio),
		ManualRevisionNumber:            manualRevision,
		RecordedAt:                      now,
	}); err != nil {
		return err
	}
	for _, balance := range report.Balances {
		if !decimal.Valid(balance.Equity) {
			return fmt.Errorf("%w: balance %s equity %q", catalog.ErrInvalid, balance.AssetSymbol, balance.Equity)
		}
		assetID, _, err := s.catalog.EnsureAsset(ctx, q, portfolioID, platform, balance.AssetSymbol, observed)
		if err != nil {
			return err
		}
		if err := q.InsertBalance(ctx, sqlcgen.InsertBalanceParams{
			ReportID: id[:], PortfolioID: portfolioID[:], AssetID: assetID[:], Equity: balance.Equity,
			AvailableToWithdraw: decimalColumn(balance.AvailableToWithdraw), AvailableToTrade: decimalColumn(balance.AvailableToTrade),
			Liability: decimalColumn(balance.Liability), Interest: decimalColumn(balance.Interest), UnrealizedPnl: decimalColumn(balance.UnrealizedPnl),
		}); err != nil {
			return err
		}
	}
	for i := range report.Positions {
		position := &report.Positions[i]
		quantity, valid := decimal.Parse(position.SourceQuantity)
		if !decimal.Valid(position.SourceQuantity) || !valid || quantity.Sign() <= 0 || position.PositionKey == "" || len(position.PositionKey) > 255 {
			return fmt.Errorf("%w: position %s", catalog.ErrInvalid, position.FuturesContract.Symbol)
		}
		futuresContractID, err := s.catalog.EnsureFuturesContract(ctx, q, portfolioID, platform, &position.FuturesContract, observed)
		if err != nil {
			return err
		}
		if err := q.InsertPosition(ctx, sqlcgen.InsertPositionParams{
			ReportID: id[:], PortfolioID: portfolioID[:], PositionKey: position.PositionKey, FuturesContractID: futuresContractID[:],
			PositionMode: position.PositionMode, MarginSetting: nonEmptyStringColumn(position.MarginSetting),
			MarginAssetSymbol: nonEmptyStringColumn(position.MarginAssetSymbol), MarginBasis: nonEmptyStringColumn(position.MarginBasis),
			Side: position.Side, SourceQuantity: position.SourceQuantity, SourceQuantityUnit: position.SourceQuantityUnit,
			EntryPrice: decimalColumn(position.EntryPrice), MarkPrice: decimalColumn(position.MarkPrice), LiquidationPrice: decimalColumn(position.LiquidationPrice),
			UnrealizedPnl: decimalColumn(position.UnrealizedPnl), Leverage: decimalColumn(position.Leverage), Margin: decimalColumn(position.Margin),
		}); err != nil {
			return err
		}
	}
	for i := range report.DualPositions {
		position := &report.DualPositions[i]
		if report.AccountType != adapters.TypeDualInvestment {
			return fmt.Errorf("%w: dual positions outside Dual Investment", catalog.ErrInvalid)
		}
		p, err := adapters.NormalizeDualPosition("Account Report", position)
		if err != nil {
			return err
		}
		var expiry, subscribed sql.NullTime
		if p.ExpiryAt != nil {
			expiry = sql.NullTime{Time: *p.ExpiryAt, Valid: true}
		}
		if p.SubscribedAt != nil {
			subscribed = sql.NullTime{Time: *p.SubscribedAt, Valid: true}
		}
		if err := q.InsertDualPosition(ctx, sqlcgen.InsertDualPositionParams{
			ReportID: id[:], PortfolioID: portfolioID[:], PositionKey: p.PositionKey,
			ProductRef: stringColumn(p.ProductRef), OrderRef: stringColumn(p.OrderRef), BaseAssetSymbol: stringColumn(p.BaseAssetSymbol), QuoteAssetSymbol: stringColumn(p.QuoteAssetSymbol),
			PrincipalAssetSymbol: p.PrincipalAssetSymbol, Principal: p.Principal, Direction: stringColumn(p.Direction), TargetPrice: decimalColumn(p.TargetPrice), ExpiryAt: expiry,
			AnnualizedRate: decimalColumn(p.AnnualizedRate), RateKind: stringColumn(p.RateKind), Status: p.Status,
			SubscribedAt: subscribed,
		}); err != nil {
			return err
		}
	}

	for i := range report.EarnPositions {
		p := &report.EarnPositions[i]
		payload, err := json.Marshal(adapters.EarnDetails{Assets: p.Assets, Rates: p.Rates, Rewards: p.Rewards, Details: p.Details, Issues: p.Problems})
		if err != nil {
			return err
		}
		if err := q.InsertEarnPosition(ctx, sqlcgen.InsertEarnPositionParams{ReportID: id[:], PortfolioID: portfolioID[:], PositionKey: p.PositionKey, ProductRef: stringColumn(p.ProductRef), ProductName: stringColumn(p.ProductName), ProductType: p.Type, Granularity: p.Granularity, AmountBasis: p.AmountBasis, Status: stringColumn(p.Status), SubscribedAt: timeColumn(p.SubscribedAt), MaturityAt: timeColumn(p.MaturityAt), DetailsJson: sqljson.Column(payload)}); err != nil {
			return err
		}
	}
	return nil
}

func decimalColumn(value *string) sql.NullString {
	if value == nil || !decimal.Valid(*value) {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func stringColumn(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func nonEmptyStringColumn(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func timeColumn(at *time.Time) sql.NullTime {
	if at == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: at.UTC(), Valid: true}
}
