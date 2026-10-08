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

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
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
	Manual      *manual.Service
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

// RunCycle collects every enabled Identity of the Portfolio for jobID and writes the Valuation Point.
func (s *Service) RunCycle(ctx context.Context, portfolioID, jobID uuid.UUID) (Summary, error) {
	point, err := s.valuation.PointByJob(ctx, portfolioID, jobID)
	if err == nil {
		return s.cycleSummary(ctx, point)
	}
	if !errors.Is(err, valuation.ErrNotFound) {
		return Summary{}, err
	}
	reports, err := s.q.ListReportsByJob(ctx, sqlcgen.ListReportsByJobParams{PortfolioID: portfolioID[:], JobID: jobID[:]})
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
	manualInputs, err := s.manual.Freeze(ctx, portfolioID, jobID)
	if err != nil {
		return Summary{}, err
	}
	if err := s.collectManual(ctx, portfolioID, jobID, manualInputs, reported); err != nil {
		return Summary{}, err
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
		written, err := s.collectIdentity(ctx, portfolioID, jobID, &item)
		if err != nil {
			return Summary{}, err
		}
		for id := range written {
			reported[id] = true
		}
		collected = true
	}
	now := s.now().UTC()
	for i := range boundary {
		account := &boundary[i]
		if reported[account.ID] || account.AccountType == adapters.TypeManual {
			continue
		}
		detail := "The identity's connection is " + account.ConnectionStatus + "; the account was not collected this cycle."
		if err := insertFailedReport(ctx, s.q, portfolioID, jobID, account.ID, ClassSkipped, detail, now); err != nil {
			return Summary{}, err
		}
	}
	point, err = s.valuation.WritePoint(ctx, portfolioID, jobID, s.now())
	if err != nil {
		return Summary{}, err
	}
	summary, err := s.cycleSummary(ctx, point)
	if err != nil {
		return Summary{}, err
	}
	if point.IncompleteAccountCount > 0 || summary.Invalidated > 0 {
		s.logger.WarnContext(ctx, "valuation point written with issues",
			slog.String("portfolio_id", portfolioID.String()), slog.String("job_id", jobID.String()),
			slog.Int("incomplete_accounts", point.IncompleteAccountCount), slog.Int("stale_accounts", point.StaleAccountCount),
			slog.Int("connections_invalidated", summary.Invalidated))
	}
	return summary, nil
}

// collectIdentity calls the adapter with bounded retries and writes one report per selected Account.
func (s *Service) collectIdentity(ctx context.Context, portfolioID, jobID uuid.UUID, item *identity.Collectable) (map[uuid.UUID]bool, error) {
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
	now := s.now().UTC()
	invalidate := false
	if err := mysql.ExecTx(ctx, s.db, func(q *sqlcgen.Queries) error {
		if class != "" {
			invalidate = adapters.StopsScheduling(class)
			for _, a := range item.Accounts {
				if err := insertFailedReport(ctx, q, portfolioID, jobID, a.ID, class, detail, now); err != nil {
					return err
				}
				written[a.ID] = true
			}
		} else {
			if err := s.identities.RecordMarginMode(ctx, q, portfolioID, item.Identity.ID, batch.MarginMode); err != nil {
				return err
			}
			failed := 0
			if err := s.storeBatch(ctx, q, portfolioID, jobID, item, &batch, byType, written, &failed, now); err != nil {
				return err
			}
			for _, a := range item.Accounts {
				if !written[a.ID] {
					if err := insertFailedReport(ctx, q, portfolioID, jobID, a.ID, adapters.ClassNormalization, "The adapter returned no report for this account.", now); err != nil {
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
		return s.identities.RecordCollection(ctx, q, portfolioID, item.Identity.ID, now, class, detail, invalidate)
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
