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

func (s *Service) storeBatch(ctx context.Context, q *sqlcgen.Queries, portfolioID, jobID uuid.UUID, item *identity.Collectable, batch *adapters.Batch, byType map[adapters.AccountType]identity.Account, written map[uuid.UUID]bool, failed *int, now time.Time) error {
	for _, price := range batch.Prices {
		if price.Price == "" {
			continue
		}
		if price.ObservedAt.IsZero() {
			price.ObservedAt = batch.ObservedAt
		}
		if err := s.catalog.RecordPrice(ctx, q, item.Identity.Platform, price); err != nil {
			if errors.Is(err, catalog.ErrInvalid) {
				continue
			}
			return err
		}
	}
	for i := range batch.Reports {
		report := &batch.Reports[i]
		account, ok := byType[report.AccountType]
		if !ok || written[account.ID] {
			continue
		}
		if report.Status != adapters.StatusOK {
			class := report.ErrorClass
			if class == "" {
				class = adapters.ClassTemporary
			}
			if err := insertFailedReport(ctx, q, portfolioID, jobID, account.ID, class, report.ErrorDetail, now); err != nil {
				return err
			}
			written[account.ID] = true
			*failed++
			continue
		}
		if err := s.insertSuccessfulReport(ctx, q, portfolioID, jobID, item.Identity.Platform, account.ID, report, batch.MarginMode, batch.ObservedAt, now); err != nil {
			return err
		}
		written[account.ID] = true
	}
	return nil
}

func insertFailedReport(ctx context.Context, q *sqlcgen.Queries, portfolioID, jobID, accountID uuid.UUID, class, detail string, now time.Time) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	if detail == "" {
		detail = "The platform call failed."
	}
	account, err := q.GetAccount(ctx, sqlcgen.GetAccountParams{PortfolioID: portfolioID[:], ID: accountID[:]})
	if err != nil {
		return err
	}
	return q.InsertAccountReport(ctx, sqlcgen.InsertAccountReportParams{
		ID: id[:], PortfolioID: portfolioID[:], JobID: jobID[:], AccountID: accountID[:], ScopeVersion: adapters.ReportScope(adapters.Platform(account.Platform), adapters.AccountType(account.AccountType)), Status: adapters.StatusFailed,
		ErrorClass: sql.NullString{String: class, Valid: true}, ErrorDetail: sql.NullString{String: truncate(detail, 500), Valid: true},
		CreatedAt: now,
	})
}

func (s *Service) insertSuccessfulReport(ctx context.Context, q *sqlcgen.Queries, portfolioID, jobID uuid.UUID, platform adapters.Platform, accountID uuid.UUID, report *adapters.AccountReport, mode adapters.MarginMode, batchObserved, now time.Time) error {
	if report.EarnDetailsCollected || len(report.EarnPositions) > 0 {
		if report.AccountType != "earn" || !report.EarnDetailsCollected {
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
	observed := report.ObservedAt
	if observed.IsZero() {
		observed = batchObserved
	}
	if observed.IsZero() {
		observed = now
	}
	if err := q.InsertAccountReport(ctx, sqlcgen.InsertAccountReportParams{
		ID: id[:], PortfolioID: portfolioID[:], JobID: jobID[:], AccountID: accountID[:], ScopeVersion: adapters.ReportScope(platform, report.AccountType), Status: adapters.StatusOK,
		EarnDetailsCollected:   report.EarnDetailsCollected,
		ObservedAt:             sql.NullTime{Time: observed.UTC(), Valid: true},
		MarginMode:             nonEmptyStringColumn(string(mode)),
		Equity:                 decimalColumn(report.Equity),
		AdjustedEquity:         decimalColumn(report.AdjustedEquity),
		AvailableEquity:        decimalColumn(report.AvailableEquity),
		InitialMarginRatio:     decimalColumn(report.InitialMarginRatio),
		MaintenanceMarginRatio: decimalColumn(report.MaintenanceMarginRatio),
		UnifiedMarginRatio:     decimalColumn(report.UnifiedMarginRatio),
		CreatedAt:              now,
	}); err != nil {
		return err
	}
	for _, balance := range report.Balances {
		if !decimal.Valid(balance.Equity) {
			return fmt.Errorf("%w: balance %s equity %q", catalog.ErrInvalid, balance.PlatformAssetSymbol, balance.Equity)
		}
		platformAssetID, _, err := s.catalog.EnsurePlatformAsset(ctx, q, platform, balance.PlatformAssetSymbol, balance.Name, observed)
		if err != nil {
			return err
		}
		if err := q.InsertBalance(ctx, sqlcgen.InsertBalanceParams{
			ReportID: id[:], PortfolioID: portfolioID[:], PlatformAssetID: platformAssetID[:], Equity: balance.Equity,
			AvailableToWithdraw: decimalColumn(balance.AvailableToWithdraw), AvailableToTrade: decimalColumn(balance.AvailableToTrade),
			Liability: decimalColumn(balance.Liability), Interest: decimalColumn(balance.Interest), UnrealizedPnl: decimalColumn(balance.UnrealizedPnl),
		}); err != nil {
			return err
		}
	}
	for i := range report.Positions {
		position := &report.Positions[i]
		if !decimal.Valid(position.Quantity) || position.PlatformPositionRef == "" {
			return fmt.Errorf("%w: position %s", catalog.ErrInvalid, position.FuturesContract.Symbol)
		}
		futuresContractID, err := s.catalog.EnsureFuturesContract(ctx, q, platform, &position.FuturesContract, observed)
		if err != nil {
			return err
		}
		if err := q.InsertPosition(ctx, sqlcgen.InsertPositionParams{
			ReportID: id[:], PortfolioID: portfolioID[:], PlatformPositionRef: truncate(position.PlatformPositionRef, 128), FuturesContractID: futuresContractID[:],
			PositionMode: nonEmptyStringColumn(position.PositionMode), MarginMode: nonEmptyStringColumn(position.MarginMode),
			MarginPlatformAssetSymbol: nonEmptyStringColumn(position.MarginCurrency), MarginBasis: nonEmptyStringColumn(position.MarginBasis),
			Side: position.Side, Quantity: position.Quantity, QuantityUnit: position.QuantityUnit,
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
			ReportID: id[:], PortfolioID: portfolioID[:], PlatformPositionRef: p.PlatformPositionRef,
			ProductRef: stringColumn(p.ProductRef), OrderRef: stringColumn(p.OrderRef), BasePlatformAssetSymbol: stringColumn(p.BaseSymbol), QuotePlatformAssetSymbol: stringColumn(p.QuoteSymbol),
			InvestPlatformAssetSymbol: p.InvestSymbol, Principal: p.Principal, Direction: stringColumn(p.Direction), TargetPrice: decimalColumn(p.TargetPrice), ExpiryAt: expiry,
			AnnualizedRate: decimalColumn(p.AnnualizedRate), RateKind: stringColumn(p.RateKind), Status: p.Status,
			SubscribedAt: subscribed,
		}); err != nil {
			return err
		}
	}

	for i := range report.EarnPositions {
		p := &report.EarnPositions[i]
		payload, err := json.Marshal(p)
		if err != nil {
			return err
		}
		if err := q.InsertEarnPosition(ctx, sqlcgen.InsertEarnPositionParams{ReportID: id[:], PortfolioID: portfolioID[:], PlatformPositionRef: p.PlatformPositionRef, ObservationJson: sqljson.Column(payload)}); err != nil {
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
