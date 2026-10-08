package jobs

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// ClaimOne leases one available job for workerID. ok is false when the queue is idle.
func (s *Service) ClaimOne(ctx context.Context, workerID string, lease time.Duration) (ClaimedJob, bool, error) {
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ClaimedJob{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	row, err := q.LockClaimableJob(ctx, sqlcgen.LockClaimableJobParams{QueuedBefore: now, RunningBefore: sql.NullTime{Time: now, Valid: true}})
	if errors.Is(err, sql.ErrNoRows) {
		return ClaimedJob{}, false, nil
	}
	if err != nil {
		return ClaimedJob{}, false, err
	}
	job, err := jobFromRow((*sqlcgen.GetJobRow)(&row))
	if err != nil {
		return ClaimedJob{}, false, err
	}
	attempt := job.AttemptCount + 1
	if attempt > job.MaxAttempts {
		if _, err := q.MarkJobFailed(ctx, sqlcgen.MarkJobFailedParams{
			LastErrorClass: sql.NullString{String: "max_attempts", Valid: true},
			LastDetail:     sql.NullString{String: "The job exceeded its retry budget.", Valid: true},
			FinishedAt:     sql.NullTime{Time: now, Valid: true},
			UpdatedAt:      now,
			ID:             job.ID[:],
			PortfolioID:    job.PortfolioID[:],
			LeaseOwner:     row.LeaseOwner,
		}); err != nil {
			return ClaimedJob{}, false, err
		}
		return ClaimedJob{}, false, tx.Commit()
	}
	if _, err := q.MarkJobRunning(ctx, sqlcgen.MarkJobRunningParams{
		AttemptCount:   attempt,
		LeaseOwner:     sql.NullString{String: workerID, Valid: true},
		LeaseExpiresAt: sql.NullTime{Time: now.Add(lease), Valid: true},
		StartedAt:      sql.NullTime{Time: now, Valid: true},
		UpdatedAt:      now,
		ID:             job.ID[:],
		PortfolioID:    job.PortfolioID[:],
	}); err != nil {
		return ClaimedJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ClaimedJob{}, false, err
	}
	job.Status = StatusRunning
	job.AttemptCount = attempt
	job.StartedAt = &now
	return ClaimedJob{Job: job, WorkerID: workerID}, true, nil
}

// RenewLease extends a running job lease.
func (s *Service) RenewLease(ctx context.Context, job *ClaimedJob, lease time.Duration) error {
	now := s.now().UTC()
	n, err := s.q.RenewJobLease(ctx, sqlcgen.RenewJobLeaseParams{
		LeaseExpiresAt: sql.NullTime{Time: now.Add(lease), Valid: true},
		UpdatedAt:      now,
		ID:             job.ID[:],
		PortfolioID:    job.PortfolioID[:],
		LeaseOwner:     sql.NullString{String: job.WorkerID, Valid: true},
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLostLease
	}
	return nil
}

// Finish records the terminal or retry state of a claimed job.
func (s *Service) Finish(ctx context.Context, job *ClaimedJob, result Result) error {
	now := s.now().UTC()
	owner := sql.NullString{String: job.WorkerID, Valid: true}
	var n int64
	var err error
	switch {
	case result.OK:
		n, err = s.q.MarkJobSucceeded(ctx, sqlcgen.MarkJobSucceededParams{
			LastDetail:  nullable(result.Detail),
			FinishedAt:  sql.NullTime{Time: now, Valid: true},
			UpdatedAt:   now,
			ID:          job.ID[:],
			PortfolioID: job.PortfolioID[:],
			LeaseOwner:  owner,
		})
	case result.Retry && job.AttemptCount < job.MaxAttempts:
		retryAt := result.RetryAt
		if retryAt.IsZero() {
			retryAt = now
		}
		n, err = s.q.RequeueJob(ctx, sqlcgen.RequeueJobParams{
			AvailableAt:    retryAt.UTC(),
			LastErrorClass: nullable(result.ErrorClass),
			LastDetail:     nullable(result.Detail),
			UpdatedAt:      now,
			ID:             job.ID[:],
			PortfolioID:    job.PortfolioID[:],
			LeaseOwner:     owner,
		})
	default:
		n, err = s.q.MarkJobFailed(ctx, sqlcgen.MarkJobFailedParams{
			LastErrorClass: nullable(result.ErrorClass),
			LastDetail:     nullable(result.Detail),
			FinishedAt:     sql.NullTime{Time: now, Valid: true},
			UpdatedAt:      now,
			ID:             job.ID[:],
			PortfolioID:    job.PortfolioID[:],
			LeaseOwner:     owner,
		})
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLostLease
	}
	return nil
}
