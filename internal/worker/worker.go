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

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/jobs"
)

const (
	emptyProjectionDetail = "empty projection: no source facts; active generation unchanged"
	maxBackoff            = time.Minute
)

// ErrUnavailable means the worker loop is not heartbeating.
var ErrUnavailable = errors.New("worker_unavailable")

// Worker claims durable jobs and runs Phase 1 handlers.
type Worker struct {
	jobs     *jobs.Service
	adapters *adapters.Registry
	poll     time.Duration
	lease    time.Duration
	id       string
	logger   *slog.Logger
	lastBeat atomic.Int64
}

// Options configures a worker.
type Options struct {
	Jobs     *jobs.Service
	Adapters *adapters.Registry
	Poll     time.Duration
	Lease    time.Duration
	Logger   *slog.Logger
	ID       string
}

// New constructs a worker. The adapter registry may be empty.
func New(opts Options) *Worker {
	if opts.Poll <= 0 {
		opts.Poll = time.Second
	}
	if opts.Lease <= 0 {
		opts.Lease = 2 * time.Minute
	}
	if opts.Adapters == nil {
		opts.Adapters = adapters.NewRegistry()
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	id := opts.ID
	if id == "" {
		id = defaultWorkerID()
	}
	return &Worker{
		jobs:     opts.Jobs,
		adapters: opts.Adapters,
		poll:     opts.Poll,
		lease:    opts.Lease,
		id:       id,
		logger:   opts.Logger,
	}
}

// Run claims work until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	w.beat()
	ticker := time.NewTicker(w.poll)
	defer ticker.Stop()
	for {
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
	result := w.handle(ctx, &claimed)
	if err := w.jobs.FinishAttempt(ctx, &claimed, result); err != nil {
		return true, err
	}
	return true, nil
}

func (w *Worker) handle(ctx context.Context, claimed *jobs.ClaimedJob) jobs.Result {
	renewCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go w.renewLoop(renewCtx, claimed)

	switch claimed.Kind {
	case jobs.KindCollection:
		return w.handleCollection()
	case jobs.KindProjection:
		return w.handleProjection(ctx, claimed)
	default:
		return jobs.Result{
			ErrorClass: "unsupported",
			ErrorCode:  "unknown_kind",
			SafeDetail: "The job kind is not supported.",
		}
	}
}

func (w *Worker) handleCollection() jobs.Result {
	if w.adapters.Empty() {
		return jobs.Result{
			ErrorClass: "unsupported",
			ErrorCode:  "adapter_unavailable",
			SafeDetail: "No Platform adapter is registered.",
		}
	}
	return jobs.Result{
		ErrorClass: "unsupported",
		ErrorCode:  "collection_not_enabled",
		SafeDetail: "Collection is not enabled in this phase.",
	}
}

func (w *Worker) handleProjection(ctx context.Context, claimed *jobs.ClaimedJob) jobs.Result {
	if _, err := w.jobs.ActiveGenerationID(ctx, claimed.PortfolioID); err != nil {
		retryAt := retryTime(claimed.AttemptNumber, time.Now().UTC())
		return jobs.Result{
			Retry:      true,
			RetryAt:    &retryAt,
			ErrorClass: "temporary",
			ErrorCode:  "generation_pointer_read",
			SafeDetail: "The portfolio generation pointer could not be read.",
		}
	}
	// Phase 1 leaves a diagnostic empty result and never switches the active generation.
	return jobs.Result{OK: true, SafeDetail: emptyProjectionDetail}
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
