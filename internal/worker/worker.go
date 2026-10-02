// Package worker claims durable jobs, runs Collection Cycles and recalculations, and schedules cycles.
package worker

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/collection"
	"github.com/rbee3u/plutus/internal/infra/logging"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/valuation"
)

const (
	defaultPollInterval  = time.Second
	defaultLeaseDuration = 2 * time.Minute
	maxBackoff           = time.Minute
	schedulerInterval    = 15 * time.Second
	pruneInterval        = time.Hour
)

// ErrUnavailable means the worker loop is not heartbeating.
var ErrUnavailable = errors.New("worker_unavailable")

// Worker claims durable jobs and runs the handlers.
type Worker struct {
	jobs       *jobs.Service
	collection *collection.Service
	valuation  *valuation.Service
	poll       time.Duration
	lease      time.Duration
	id         string
	logger     *slog.Logger
	lastBeat   atomic.Int64
	scheduler  bool
	retention  time.Duration
}

// Options configures a worker.
type Options struct {
	Jobs       *jobs.Service
	Collection *collection.Service
	Valuation  *valuation.Service
	Poll       time.Duration
	Lease      time.Duration
	Logger     *slog.Logger
	ID         string
	// Scheduler enables the per-Portfolio Collection Cycle scheduler.
	Scheduler bool
	// Retention prunes finished jobs without reports or points after this age; zero disables pruning.
	Retention time.Duration
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
	return &Worker{jobs: opts.Jobs, collection: opts.Collection, valuation: opts.Valuation, poll: opts.Poll, lease: opts.Lease, id: id, logger: opts.Logger, scheduler: opts.Scheduler, retention: opts.Retention}
}

// Run claims work and schedules cycles until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	w.beat()
	ticker := time.NewTicker(w.poll)
	defer ticker.Stop()
	var lastSchedule, lastPrune time.Time
	for {
		if w.scheduler && time.Since(lastSchedule) >= schedulerInterval {
			if err := w.Schedule(ctx); err != nil && !errors.Is(err, context.Canceled) {
				w.logger.Error("scheduler tick failed", slog.Any("err", err))
			}
			lastSchedule = time.Now()
		}
		if w.scheduler && w.retention > 0 && time.Since(lastPrune) >= pruneInterval {
			if err := w.Prune(ctx); err != nil && !errors.Is(err, context.Canceled) {
				w.logger.Error("job pruning failed", slog.Any("err", err))
			}
			lastPrune = time.Now()
		}
		claimed, err := w.ProcessOnce(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Error("job worker iteration failed", slog.Any("err", err), slog.String("worker_id", w.id))
		}
		w.beat()
		if claimed {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Schedule enqueues one Collection Cycle for every Portfolio whose interval elapsed.
func (w *Worker) Schedule(ctx context.Context) error {
	if w.jobs == nil {
		return nil
	}
	rows, err := w.jobs.ListForScheduling(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, row := range rows {
		if row.LastCycleAt != nil && now.Sub(*row.LastCycleAt) < row.Interval {
			continue
		}
		if _, created, err := w.jobs.EnqueueCycle(ctx, row.PortfolioID, jobs.TriggerSchedule, nil); err != nil {
			return err
		} else if created {
			w.logger.Info("collection cycle scheduled", slog.String("portfolio_id", row.PortfolioID.String()))
		}
	}
	return nil
}

// Prune deletes finished jobs older than the retention that hold no reports or points.
func (w *Worker) Prune(ctx context.Context) error {
	if w.jobs == nil || w.retention <= 0 {
		return nil
	}
	deleted, err := w.jobs.PruneFinished(ctx, time.Now().UTC().Add(-w.retention))
	if err != nil {
		return err
	}
	if deleted > 0 {
		w.logger.Info("finished jobs pruned", slog.Int64("deleted", deleted))
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

// ProcessOnce claims and executes at most one job.
func (w *Worker) ProcessOnce(ctx context.Context) (bool, error) {
	if w.jobs == nil {
		return false, nil
	}
	claimed, ok, err := w.jobs.ClaimOne(ctx, w.id, w.lease)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	ctx = logging.WithFields(ctx, slog.String("job_id", claimed.ID.String()), slog.String("portfolio_id", claimed.PortfolioID.String()))
	if claimed.Kind == jobs.KindCollectionCycle {
		ctx = logging.WithFields(ctx, slog.String("cycle_id", claimed.ID.String()))
	}
	result := w.handle(ctx, &claimed)
	if err := w.jobs.Finish(ctx, &claimed, result); err != nil {
		return true, err
	}
	return true, nil
}

func (w *Worker) handle(ctx context.Context, claimed *jobs.ClaimedJob) jobs.Result {
	renewCtx, cancel := context.WithCancel(ctx)
	renewDone := make(chan struct{})
	go func() { defer close(renewDone); w.renewLoop(renewCtx, claimed) }()
	defer func() { cancel(); <-renewDone }()

	switch claimed.Kind {
	case jobs.KindCollectionCycle:
		return w.handleCycle(ctx, claimed)
	case jobs.KindValuationRecalc:
		return w.handleRecalc(ctx, claimed)
	default:
		return jobs.Result{ErrorClass: "unsupported", Detail: "The job kind is not supported."}
	}
}

func (w *Worker) handleCycle(ctx context.Context, claimed *jobs.ClaimedJob) jobs.Result {
	if w.collection == nil {
		return jobs.Result{ErrorClass: "unsupported", Detail: "Collection is not configured."}
	}
	summary, err := w.collection.RunCycle(ctx, claimed.PortfolioID, claimed.ID)
	if err != nil {
		w.logger.ErrorContext(ctx, "collection cycle failed", slog.String("job_id", claimed.ID.String()), slog.Any("err", err))
		return jobs.Result{Retry: true, RetryAt: retryTime(claimed.AttemptCount, time.Now().UTC()), ErrorClass: "temporary", Detail: safeDetail(err)}
	}
	return jobs.Result{OK: true, Detail: summary.String()}
}

func (w *Worker) handleRecalc(ctx context.Context, claimed *jobs.ClaimedJob) jobs.Result {
	if w.valuation == nil {
		return jobs.Result{ErrorClass: "unsupported", Detail: "Valuation is not configured."}
	}
	count, err := w.valuation.Rebuild(ctx, claimed.PortfolioID)
	if err != nil {
		w.logger.ErrorContext(ctx, "valuation recalculation failed", slog.String("job_id", claimed.ID.String()), slog.Any("err", err))
		return jobs.Result{Retry: true, RetryAt: retryTime(claimed.AttemptCount, time.Now().UTC()), ErrorClass: "temporary", Detail: safeDetail(err)}
	}
	return jobs.Result{OK: true, Detail: fmt.Sprintf("recalculated %d valuation points", count)}
}

func (w *Worker) renewLoop(ctx context.Context, claimed *jobs.ClaimedJob) {
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
			if err := w.jobs.RenewLease(ctx, claimed, w.lease); err != nil {
				return
			}
		}
	}
}

// safeDetail keeps job details short and free of payloads.
func safeDetail(err error) string {
	msg := err.Error()
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return msg
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
