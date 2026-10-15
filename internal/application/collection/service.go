// Package collection runs one collection cycle: adapter calls, account reports, and the valuation point.
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

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/cycles"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// IdentityDelay is the fixed runtime delay between identities in one collection cycle.
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
	Manual      *manual.Service
	Valuation   *valuation.Service
	Stagger     time.Duration
	RetryDelays []time.Duration
	Logger      *slog.Logger
}

// Service runs collection cycles.
type Service struct {
	db          *sql.DB
	q           *sqlcgen.Queries
	now         func() time.Time
	sleep       func(context.Context, time.Duration) error
	adapters    *adapters.Registry
	identities  *identity.Service
	manual      *manual.Service
	catalog     *catalog.Service
	valuation   *valuation.Service
	stagger     time.Duration
	retryDelays []time.Duration
	logger      *slog.Logger
}

// NewService constructs a cycle runner.
func NewService(db *sql.DB, opts Options) *Service {
	s := &Service{
		db: db, q: sqlcgen.New(db), now: time.Now, sleep: sleepCtx, manual: opts.Manual,
		adapters: opts.Adapters, identities: opts.Identities, catalog: opts.Catalog, valuation: opts.Valuation,
		stagger: opts.Stagger, retryDelays: opts.RetryDelays, logger: opts.Logger,
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

// RunCycle collects every enabled identity of the portfolio for cycleID and writes the valuation point.
func (s *Service) RunCycle(ctx context.Context, portfolioID, cycleID uuid.UUID, claim ...*cycles.Claim) (Summary, error) {
	point, err := s.valuation.PointByCycle(ctx, portfolioID, cycleID)
	if err == nil {
		summary, err := s.cycleSummary(ctx, point)
		if err != nil {
			return Summary{}, err
		}
		if len(claim) > 0 && claim[0] != nil {
			if err := cycles.NewService(s.db).Finish(ctx, claim[0], cycles.Result{OK: true}); err != nil {
				return Summary{}, err
			}
		}
		return summary, nil
	}
	if !errors.Is(err, valuation.ErrNotFound) {
		return Summary{}, err
	}
	completed, err := s.q.GetCollectionCycle(ctx, sqlcgen.GetCollectionCycleParams{PortfolioID: portfolioID[:], ID: cycleID[:]})
	if err != nil {
		return Summary{}, err
	}
	if completed.CollectionFinishedAt.Valid {
		point, err := s.valuation.WritePoint(ctx, portfolioID, cycleID, completed.CollectionFinishedAt.Time, claim...)
		if err != nil {
			return Summary{}, err
		}
		return s.cycleSummary(ctx, point)
	}
	reports, err := s.q.ListReportsByCycle(ctx, sqlcgen.ListReportsByCycleParams{PortfolioID: portfolioID[:], CycleID: cycleID[:]})
	if err != nil {
		return Summary{}, err
	}
	reported := map[uuid.UUID]bool{}
	for i := range reports {
		id, err := uuid.FromBytes(reports[i].AccountID)
		if err != nil {
			return Summary{}, err
		}
		reported[id] = true
	}
	prices := recordedPrices{}
	if !completed.ManualReportedAt.Valid {
		if err := s.collectManual(ctx, portfolioID, cycleID, reported, prices); err != nil {
			return Summary{}, err
		}
	}
	boundary, err := s.identities.ListBoundary(ctx, portfolioID)
	if err != nil {
		return Summary{}, err
	}
	collectable, err := s.identities.ListCollectable(ctx, portfolioID)
	if err != nil {
		return Summary{}, err
	}
	collected := false
	for i := range collectable {
		item := collectable[i]
		item.Accounts = nil
		for _, account := range collectable[i].Accounts {
			if !reported[account.ID] {
				item.Accounts = append(item.Accounts, account)
			}
		}
		if len(item.Accounts) == 0 {
			continue
		}
		if collected && s.stagger > 0 {
			if err := s.sleep(ctx, s.stagger); err != nil {
				return Summary{}, err
			}
		}
		written, err := s.collectIdentity(ctx, portfolioID, cycleID, &item, prices)
		if err != nil {
			return Summary{}, err
		}
		for id := range written {
			reported[id] = true
		}
		collected = true
	}
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return err
		}
		now := s.now().UTC()
		for i := range boundary {
			account := &boundary[i]
			if reported[account.ID] || account.AccountType == adapters.TypeManual {
				continue
			}
			detail := "The identity's connection is " + account.ConnectionStatus + "; the account was not collected this cycle."
			if err := insertFailedReport(ctx, q, portfolioID, cycleID, account.ID, ClassSkipped, detail, now); err != nil {
				return err
			}
		}
		if err := q.MarkCollectionCycleCollected(ctx, sqlcgen.MarkCollectionCycleCollectedParams{PortfolioID: portfolioID[:], ID: cycleID[:], At: sql.NullTime{Time: s.now().UTC(), Valid: true}}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return Summary{}, err
	}
	cycle, err := s.q.GetCollectionCycle(ctx, sqlcgen.GetCollectionCycleParams{PortfolioID: portfolioID[:], ID: cycleID[:]})
	if err != nil {
		return Summary{}, err
	}
	point, err = s.valuation.WritePoint(ctx, portfolioID, cycleID, cycle.CollectionFinishedAt.Time, claim...)
	if err != nil {
		return Summary{}, err
	}
	summary, err := s.cycleSummary(ctx, point)
	if err != nil {
		return Summary{}, err
	}
	if point.IncompleteAccountCount > 0 || summary.Invalidated > 0 {
		s.logger.WarnContext(ctx, "valuation point written with issues",
			slog.String("portfolio_id", portfolioID.String()), slog.String("cycle_id", cycleID.String()),
			slog.Int("incomplete_accounts", point.IncompleteAccountCount), slog.Int("stale_accounts", point.StaleAccountCount),
			slog.Int("connections_invalidated", summary.Invalidated))
	}
	return summary, nil
}

// collectIdentity calls the adapter with bounded retries and writes one report per selected account.
func (s *Service) collectIdentity(ctx context.Context, portfolioID, cycleID uuid.UUID, item *identity.Collectable, prices recordedPrices) (map[uuid.UUID]bool, error) {
	written := map[uuid.UUID]bool{}
	if len(item.Accounts) == 0 {
		return written, nil
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
	if class == "" {
		if err := s.supplementPrices(ctx, portfolioID, item, &batch); err != nil {
			return nil, err
		}
	}
	now := s.now().UTC()
	invalidate := false
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			return err
		}
		if class != "" {
			invalidate = adapters.StopsScheduling(class)
			for _, a := range item.Accounts {
				if err := insertFailedReport(ctx, q, portfolioID, cycleID, a.ID, class, detail, now); err != nil {
					return err
				}
				written[a.ID] = true
			}
		} else {
			if err := s.identities.RecordMarginMode(ctx, q, portfolioID, item.Identity.ID, item.Identity.Version, batch.MarginMode); err != nil {
				return err
			}
			failed := 0
			if err := s.storeBatch(ctx, q, portfolioID, cycleID, item, &batch, byType, written, &failed, prices, now); err != nil {
				return err
			}
			for _, a := range item.Accounts {
				if !written[a.ID] {
					if err := insertFailedReport(ctx, q, portfolioID, cycleID, a.ID, adapters.ClassNormalization, "The adapter returned no report for this account.", now); err != nil {
						return err
					}
					written[a.ID] = true
					failed++
				}
			}
			if failed == len(item.Accounts) {
				class, detail = adapters.ClassNormalization, "Every collected account failed this cycle."
			}
		}
		return s.identities.RecordCollection(ctx, q, portfolioID, item.Identity.ID, item.Identity.Version, now, class, detail, invalidate)
	}); err != nil {
		return nil, err
	}
	if invalidate {
		s.logger.WarnContext(ctx, "connection set invalid", slog.String("identity_id", item.Identity.ID.String()), slog.String("error_class", class))
	}
	return written, nil
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

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
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
