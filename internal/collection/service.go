// Package collection runs one Collection Cycle: adapter calls, Account Reports, and the Valuation Point.
package collection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/valuation"
)

// IdentityDelay is the fixed runtime delay between Identities in one Collection Cycle.
const IdentityDelay = 3 * time.Second

// Error classes written by the runner itself.
const (
	ClassSkipped     = "skipped"
	ClassCredentials = "credentials"
)

// Options configures a cycle runner.
type Options struct {
	Adapters    *adapters.Registry
	Identities  *identity.Service
	Catalog     *catalog.Service
	Valuation   *valuation.Service
	Stagger     time.Duration
	RetryDelays []time.Duration
	Logger      *slog.Logger
}

// Service runs Collection Cycles.
type Service struct {
	db          *sql.DB
	q           *sqlcgen.Queries
	now         func() time.Time
	sleep       func(context.Context, time.Duration) error
	adapters    *adapters.Registry
	identities  *identity.Service
	catalog     *catalog.Service
	valuation   *valuation.Service
	stagger     time.Duration
	retryDelays []time.Duration
	logger      *slog.Logger
}

// NewService constructs a cycle runner.
func NewService(db *sql.DB, opts Options) *Service {
	s := &Service{
		db: db, q: sqlcgen.New(db), now: time.Now, sleep: sleepCtx,
		adapters: opts.Adapters, identities: opts.Identities, catalog: opts.Catalog, valuation: opts.Valuation,
		stagger: opts.Stagger, retryDelays: opts.RetryDelays, logger: opts.Logger,
	}
	if s.adapters == nil {
		s.adapters = adapters.NewRegistry()
	}
	if s.retryDelays == nil {
		s.retryDelays = []time.Duration{2 * time.Second, 5 * time.Second}
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	return s
}

// Summary is the outcome of one cycle.
type Summary struct {
	Point           valuation.Point
	IdentityCount   int
	IdentityFailed  int
	ReportCount     int
	ReportFailed    int
	Invalidated     int
	ObservationTime time.Time
}

// String renders the cycle outcome as one short sentence; failures and gaps appear only when present.
func (s Summary) String() string {
	parts := []string{
		plural(s.IdentityCount, "identity", "identities"),
		plural(s.ReportCount, "report", "reports"),
		"total " + s.Point.TotalUSD + " USD",
	}
	if s.IdentityFailed > 0 {
		parts = append(parts, plural(s.IdentityFailed, "identity failed", "identities failed"))
	}
	if s.ReportFailed > 0 {
		parts = append(parts, plural(s.ReportFailed, "report failed", "reports failed"))
	}
	if s.Point.StaleAccountCount > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", s.Point.StaleAccountCount))
	}
	if s.Point.IncompleteAccountCount > 0 {
		parts = append(parts, fmt.Sprintf("%d incomplete", s.Point.IncompleteAccountCount))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// RunCycle collects every enabled Identity of the Portfolio for jobID and writes the Valuation Point.
func (s *Service) RunCycle(ctx context.Context, portfolioID, jobID uuid.UUID) (Summary, error) {
	boundary, err := s.identities.ListBoundary(ctx, portfolioID)
	if err != nil {
		return Summary{}, err
	}
	collectable, err := s.identities.ListCollectable(ctx, portfolioID)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{IdentityCount: len(collectable)}
	reported := map[uuid.UUID]bool{}
	for i := range collectable {
		item := &collectable[i]
		if i > 0 && s.stagger > 0 {
			if err := s.sleep(ctx, s.stagger); err != nil {
				return Summary{}, err
			}
		}
		written, failed, invalidated, err := s.collectIdentity(ctx, portfolioID, jobID, item)
		if err != nil {
			return Summary{}, err
		}
		for id := range written {
			reported[id] = true
		}
		summary.ReportCount += len(written)
		summary.ReportFailed += failed
		if invalidated {
			summary.Invalidated++
		}
		if failed == len(item.Accounts) && len(item.Accounts) > 0 {
			summary.IdentityFailed++
		}
	}
	now := s.now().UTC()
	for i := range boundary {
		account := &boundary[i]
		if reported[account.ID] {
			continue
		}
		detail := "The identity's connection is " + account.ConnectionStatus + "; the account was not collected this cycle."
		if err := s.insertFailedReport(ctx, s.q, portfolioID, jobID, account.ID, ClassSkipped, detail, now); err != nil {
			return Summary{}, err
		}
		summary.ReportCount++
		summary.ReportFailed++
	}
	point, err := s.valuation.WritePoint(ctx, portfolioID, jobID, s.now())
	if err != nil {
		return Summary{}, err
	}
	summary.Point = point
	summary.ObservationTime = now
	if point.IncompleteAccountCount > 0 || summary.Invalidated > 0 {
		s.logger.WarnContext(ctx, "valuation point written with issues",
			slog.String("portfolio_id", portfolioID.String()), slog.String("job_id", jobID.String()),
			slog.Int("incomplete_accounts", point.IncompleteAccountCount), slog.Int("stale_accounts", point.StaleAccountCount),
			slog.Int("connections_invalidated", summary.Invalidated))
	}
	return summary, nil
}

// collectIdentity calls the adapter with bounded retries and writes one report per selected Account.
func (s *Service) collectIdentity(ctx context.Context, portfolioID, jobID uuid.UUID, item *identity.Collectable) (map[uuid.UUID]bool, int, bool, error) {
	written := map[uuid.UUID]bool{}
	if len(item.Accounts) == 0 {
		return written, 0, false, nil
	}
	types := make([]adapters.AccountType, 0, len(item.Accounts))
	byType := map[adapters.AccountType]identity.Account{}
	for _, a := range item.Accounts {
		types = append(types, a.AccountType)
		byType[a.AccountType] = a
	}
	batch, class, detail := s.collectWithRetry(ctx, item, types)
	if class == "" && !adapters.ValidMarginMode(item.Identity.AccountStructure, batch.MarginMode) {
		class, detail = adapters.Classify(adapters.UnknownMarginMode(item.Identity.Platform, string(batch.MarginMode)))
	}
	now := s.now().UTC()
	if class != "" {
		invalidate := adapters.StopsScheduling(class)
		for _, a := range item.Accounts {
			if err := s.insertFailedReport(ctx, s.q, portfolioID, jobID, a.ID, class, detail, now); err != nil {
				return nil, 0, false, err
			}
			written[a.ID] = true
		}
		if err := s.identities.RecordCollection(ctx, portfolioID, item.Identity.ID, now, class, detail, invalidate); err != nil {
			return nil, 0, false, err
		}
		if invalidate {
			s.logger.WarnContext(ctx, "connection set invalid", slog.String("identity_id", item.Identity.ID.String()), slog.String("error_class", class))
		}
		return written, len(item.Accounts), invalidate, nil
	}
	if err := s.identities.RecordMarginMode(ctx, portfolioID, item.Identity.ID, batch.MarginMode); err != nil {
		return nil, 0, false, err
	}
	failed := 0
	if err := s.storeBatch(ctx, portfolioID, jobID, item, &batch, byType, written, &failed, now); err != nil {
		return nil, 0, false, err
	}
	for _, a := range item.Accounts {
		if !written[a.ID] {
			if err := s.insertFailedReport(ctx, s.q, portfolioID, jobID, a.ID, adapters.ClassNormalization, "The adapter returned no report for this account.", now); err != nil {
				return nil, 0, false, err
			}
			written[a.ID] = true
			failed++
		}
	}
	errClass, errDetail := "", ""
	if failed == len(item.Accounts) {
		errClass, errDetail = adapters.ClassNormalization, "Every selected account failed this cycle."
	}
	if err := s.identities.RecordCollection(ctx, portfolioID, item.Identity.ID, now, errClass, errDetail, false); err != nil {
		return nil, 0, false, err
	}
	return written, failed, false, nil
}

func (s *Service) collectWithRetry(ctx context.Context, item *identity.Collectable, types []adapters.AccountType) (adapters.Batch, string, string) {
	if item.OpenError != nil {
		return adapters.Batch{}, ClassCredentials, "The stored credentials could not be opened: " + item.OpenError.Error()
	}
	adapter, ok := s.adapters.Lookup(item.Identity.Platform)
	if !ok {
		return adapters.Batch{}, adapters.ClassUnsupported, adapters.ErrNoAdapter.Error()
	}
	var lastClass, lastDetail string
	for attempt := 0; attempt <= len(s.retryDelays); attempt++ {
		if attempt > 0 {
			if err := s.sleep(ctx, s.retryDelays[attempt-1]); err != nil {
				return adapters.Batch{}, adapters.ClassTemporary, "The cycle was cancelled."
			}
		}
		batch, err := adapter.Collect(ctx, item.Credentials, item.Identity.AccountStructure, types)
		if err == nil {
			return batch, "", ""
		}
		lastClass, lastDetail = adapters.Classify(err)
		if !adapters.Retryable(lastClass) || ctx.Err() != nil {
			break
		}
	}
	return adapters.Batch{}, lastClass, lastDetail
}

func (s *Service) storeBatch(ctx context.Context, portfolioID, jobID uuid.UUID, item *identity.Collectable, batch *adapters.Batch, byType map[adapters.AccountType]identity.Account, written map[uuid.UUID]bool, failed *int, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
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
			if err := s.insertFailedReport(ctx, q, portfolioID, jobID, account.ID, class, report.ErrorDetail, now); err != nil {
				return err
			}
			written[account.ID] = true
			*failed++
			continue
		}
		if err := s.insertOkReport(ctx, q, portfolioID, jobID, item.Identity.Platform, account.ID, report, batch.MarginMode, batch.ObservedAt, now); err != nil {
			return err
		}
		written[account.ID] = true
	}
	return tx.Commit()
}

func (s *Service) insertFailedReport(ctx context.Context, q *sqlcgen.Queries, portfolioID, jobID, accountID uuid.UUID, class, detail string, now time.Time) error {
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

func (s *Service) insertOkReport(ctx context.Context, q *sqlcgen.Queries, portfolioID, jobID uuid.UUID, platform adapters.Platform, accountID uuid.UUID, report *adapters.AccountReport, mode adapters.MarginMode, batchObserved, now time.Time) error {
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
		ObservedAt:             sql.NullTime{Time: observed.UTC(), Valid: true},
		MarginMode:             textColumn(string(mode)),
		Equity:                 nullDecimal(report.Equity),
		AdjustedEquity:         nullDecimal(report.AdjustedEquity),
		AvailableEquity:        nullDecimal(report.AvailableEquity),
		InitialMarginRatio:     nullDecimal(report.InitialMarginRatio),
		MaintenanceMarginRatio: nullDecimal(report.MaintenanceMarginRatio),
		UnifiedMarginRatio:     nullDecimal(report.UnifiedMarginRatio),
		CreatedAt:              now,
	}); err != nil {
		return err
	}
	for _, balance := range report.Balances {
		if !decimal.Valid(balance.Equity) {
			return fmt.Errorf("%w: balance %s equity %q", catalog.ErrInvalid, balance.Symbol, balance.Equity)
		}
		listingID, _, err := s.catalog.EnsureListing(ctx, q, platform, balance.Symbol, balance.Name, observed)
		if err != nil {
			return err
		}
		if err := q.InsertBalance(ctx, sqlcgen.InsertBalanceParams{
			ReportID: id[:], PortfolioID: portfolioID[:], ListingID: listingID[:], Equity: balance.Equity,
			AvailableToWithdraw: nullDecimal(balance.AvailableToWithdraw), AvailableToTrade: nullDecimal(balance.AvailableToTrade),
			Liability: nullDecimal(balance.Liability), Interest: nullDecimal(balance.Interest), UnrealizedPnl: nullDecimal(balance.UnrealizedPnl),
		}); err != nil {
			return err
		}
	}
	for i := range report.Positions {
		position := &report.Positions[i]
		if !decimal.Valid(position.Quantity) || position.PlatformPositionRef == "" {
			return fmt.Errorf("%w: position %s", catalog.ErrInvalid, position.Instrument.Symbol)
		}
		instrumentID, err := s.catalog.EnsureInstrument(ctx, q, platform, &position.Instrument, observed)
		if err != nil {
			return err
		}
		if err := q.InsertPosition(ctx, sqlcgen.InsertPositionParams{
			ReportID: id[:], PortfolioID: portfolioID[:], PlatformPositionRef: truncate(position.PlatformPositionRef, 128), InstrumentID: instrumentID[:],
			PositionMode: textColumn(position.PositionMode), MarginMode: textColumn(position.MarginMode),
			MarginCurrency: textColumn(position.MarginCurrency), MarginBasis: textColumn(position.MarginBasis),
			Side: position.Side, Quantity: position.Quantity, QuantityUnit: position.QuantityUnit,
			EntryPrice: nullDecimal(position.EntryPrice), MarkPrice: nullDecimal(position.MarkPrice), LiquidationPrice: nullDecimal(position.LiquidationPrice),
			UnrealizedPnl: nullDecimal(position.UnrealizedPnl), Leverage: nullDecimal(position.Leverage), Margin: nullDecimal(position.Margin),
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
			ProductRef: dualString(p.ProductRef), OrderRef: dualString(p.OrderRef), BaseSymbol: dualString(p.BaseSymbol), QuoteSymbol: dualString(p.QuoteSymbol),
			InvestSymbol: p.InvestSymbol, Principal: p.Principal, Direction: dualString(p.Direction), TargetPrice: nullDecimal(p.TargetPrice), ExpiryAt: expiry,
			AnnualizedRate: nullDecimal(p.AnnualizedRate), RateKind: dualString(p.RateKind), Status: p.Status,
			SubscribedAt: subscribed,
		}); err != nil {
			return err
		}
	}

	return nil
}

func nullDecimal(value *string) sql.NullString {
	if value == nil || !decimal.Valid(*value) {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func dualString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func textColumn(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
