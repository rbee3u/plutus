// Package worker claims collection cycles, drains portfolio deletions, and schedules cycles.
package worker

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/collection"
	"github.com/rbee3u/plutus/internal/application/cycles"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/retention"
	"github.com/rbee3u/plutus/internal/infra/idempotency"
	"github.com/rbee3u/plutus/internal/infra/logging"
	"github.com/rbee3u/plutus/internal/infra/mysql"
)

const (
	defaultPollInterval  = time.Second
	defaultLeaseDuration = 2 * time.Minute
	maxBackoff           = time.Minute
	schedulerInterval    = 15 * time.Second
	pruneInterval        = time.Hour
	maintenanceIdleDelay = time.Minute
)

// ErrUnavailable means the worker loop is not heartbeating.
var ErrUnavailable = errors.New("worker_unavailable")

// Worker claims durable work and runs the handlers.
type Worker struct {
	auth        *auth.Service
	idempotency *idempotency.Store
	cycles      *cycles.Service
	collection  *collection.Service
	portfolios  *portfolio.Service
	maintenance *retention.Service
	poll        time.Duration
	lease       time.Duration
	id          string
	logger      *slog.Logger
	lastBeat    atomic.Int64
	scheduler   bool
}

// Options configures a worker.
type Options struct {
	Auth        *auth.Service
	Idempotency *idempotency.Store
	Cycles      *cycles.Service
	Collection  *collection.Service
	Portfolios  *portfolio.Service
	Maintenance *retention.Service
	Poll        time.Duration
	Lease       time.Duration
	Logger      *slog.Logger
	ID          string
	// Scheduler enables the per-portfolio collection cycle scheduler.
	Scheduler bool
}

// New constructs a worker.
func New(opts *Options) *Worker {
	if opts.Poll <= 0 {
		opts.Poll = defaultPollInterval
	}
	if opts.Lease <= 0 {
		opts.Lease = defaultLeaseDuration
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	id := opts.ID
	if id == "" {
		id = defaultWorkerID()
	}
	return &Worker{auth: opts.Auth, idempotency: opts.Idempotency, cycles: opts.Cycles, collection: opts.Collection, portfolios: opts.Portfolios, maintenance: opts.Maintenance, poll: opts.Poll, lease: opts.Lease, id: id, logger: opts.Logger, scheduler: opts.Scheduler}
}

// Run claims work and schedules cycles until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	w.beat()
	ticker := time.NewTicker(w.poll)
	defer ticker.Stop()
	var lastSchedule, lastPrune, maintenanceAt time.Time
	for {
		if w.scheduler && time.Since(lastSchedule) >= schedulerInterval {
			if err := w.Schedule(ctx); err != nil && !errors.Is(err, context.Canceled) {
				w.logger.Error("scheduler tick failed", slog.Any("err", err))
			}
			lastSchedule = time.Now()
		}
		if w.scheduler && time.Since(lastPrune) >= pruneInterval {
			if err := w.Prune(ctx); err != nil && !errors.Is(err, context.Canceled) {
				w.logger.Error("maintenance pruning failed", slog.Any("err", err))
			}
			lastPrune = time.Now()
		}
		worked, err := w.ProcessOnce(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Error("worker iteration failed", slog.Any("err", err), slog.String("worker_id", w.id))
		}
		w.beat()
		if worked {
			continue
		}
		if w.maintenance != nil && !time.Now().Before(maintenanceAt) {
			deadline := time.Now().Add(250 * time.Millisecond)
			for range 16 {
				idle, err := w.maintenance.Step(ctx, time.Now().UTC())
				if err != nil {
					w.logger.Error("history maintenance failed", slog.Any("err", err))
				}
				if err != nil || idle {
					maintenanceAt = time.Now().Add(maintenanceIdleDelay)
					break
				}
				if time.Now().After(deadline) {
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Schedule enqueues one collection cycle for every portfolio whose interval elapsed.
func (w *Worker) Schedule(ctx context.Context) error {
	if w.cycles == nil {
		return nil
	}
	rows, err := w.cycles.ListForScheduling(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, row := range rows {
		if row.LastCycleAt != nil && now.Sub(*row.LastCycleAt) < row.Interval {
			continue
		}
		if _, created, err := w.cycles.Enqueue(ctx, row.PortfolioID, cycles.TriggerSchedule); err != nil {
			return err
		} else if created {
			w.logger.Info("collection cycle scheduled", slog.String("portfolio_id", row.PortfolioID.String()))
		}
	}
	return nil
}

// Prune deletes expired sessions and idempotency records.
func (w *Worker) Prune(ctx context.Context) error {
	if w.auth != nil {
		if err := w.auth.PruneExpired(ctx); err != nil {
			return err
		}
	}
	if w.idempotency != nil {
		if err := w.idempotency.PruneExpired(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Healthy reports whether the loop has heartbeated recently.
func (w *Worker) Healthy(now time.Time) bool {
	last := w.lastBeat.Load()
	if last == 0 {
		return false
	}
	maxAge := 3 * w.poll
	if maxAge < 3*time.Second {
		maxAge = 3 * time.Second
	}
	return now.Sub(time.Unix(0, last)) <= maxAge
}

func (w *Worker) beat() {
	w.lastBeat.Store(time.Now().UnixNano())
}

// ProcessOnce runs at most one cycle attempt or one portfolio deletion batch.
// Cycles take precedence so deletions never delay collection.
func (w *Worker) ProcessOnce(ctx context.Context) (bool, error) {
	if w.cycles != nil {
		claimed, ok, err := w.cycles.ClaimOne(ctx, w.id, w.lease)
		if err != nil {
			return false, err
		}
		if ok {
			return true, w.runCycle(ctx, &claimed)
		}
	}
	if w.portfolios == nil {
		return false, nil
	}
	return w.portfolios.DeleteBatch(ctx)
}

func (w *Worker) runCycle(ctx context.Context, claimed *cycles.Claim) error {
	ctx = logging.WithFields(ctx, slog.String("portfolio_id", claimed.PortfolioID.String()), slog.String("cycle_id", claimed.ID.String()))
	result := w.attempt(ctx, claimed)
	// Point persistence completes a successful cycle atomically.
	if result.OK {
		return nil
	}
	return w.cycles.Finish(ctx, claimed, result)
}

func (w *Worker) attempt(ctx context.Context, claimed *cycles.Claim) cycles.Result {
	ctx, cancel := context.WithCancel(ctx)
	renewDone := make(chan struct{})
	go func() { defer close(renewDone); w.renewLoop(ctx, claimed, cancel) }()
	defer func() { cancel(); <-renewDone }()

	if w.collection == nil {
		return cycles.Result{ErrorClass: "unsupported", ErrorDetail: "Collection is not configured."}
	}
	summary, err := w.collection.RunCycle(ctx, claimed.PortfolioID, claimed.ID, claimed)
	if err != nil {
		w.logger.ErrorContext(ctx, "collection cycle failed", slog.Any("err", err))
		if mysql.IsDuplicate(err) {
			return cycles.Result{ErrorClass: "persistence", ErrorDetail: safeDetail(err)}
		}
		return cycles.Result{Retry: true, RetryAt: retryTime(claimed.AttemptCount, time.Now().UTC()), ErrorClass: "temporary", ErrorDetail: safeDetail(err)}
	}
	w.logger.InfoContext(ctx, "collection cycle succeeded", slog.String("summary", summary.String()))
	return cycles.Result{OK: true}
}

func (w *Worker) renewLoop(ctx context.Context, claimed *cycles.Claim, cancel context.CancelFunc) {
	interval := w.lease / 3
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.cycles.RenewLease(ctx, claimed, w.lease); err != nil {
				// Point persistence may already have completed this cycle atomically.
				current, readErr := w.cycles.Get(ctx, claimed.PortfolioID, claimed.ID)
				if readErr == nil && current.Status == cycles.StatusSucceeded {
					return
				}
				w.logger.ErrorContext(ctx, "cycle lease renewal failed", slog.Any("err", err))
				cancel()
				return
			}
		}
	}
}

// safeDetail keeps error details short, valid UTF-8, and free of payloads.
func safeDetail(err error) string {
	runes := []rune(strings.ToValidUTF8(err.Error(), "\uFFFD"))
	if len(runes) > 300 {
		runes = runes[:300]
	}
	return string(runes)
}

func retryTime(attempt uint32, now time.Time) time.Time {
	shift := attempt
	if shift > 0 {
		shift--
	}
	backoff := time.Second << min(shift, uint32(6))
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	jitter, err := rand.Int(rand.Reader, big.NewInt(int64(backoff/5)+1))
	if err != nil {
		return now.Add(backoff)
	}
	return now.Add(backoff + time.Duration(jitter.Int64()))
}

func defaultWorkerID() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "plutus"
	}
	return fmt.Sprintf("%s-%d-%s", host, os.Getpid(), uuid.NewString()[:8])
}
