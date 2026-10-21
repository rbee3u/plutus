package cycle

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Claim is a leased cycle ready for execution.
type Claim struct {
	Cycle
	WorkerID string
}

// Result is the handler outcome for one attempt.
type Result struct {
	OK          bool
	Retry       bool
	RetryAt     time.Time
	ErrorClass  string
	ErrorDetail string
}

// ClaimOne leases one available cycle for workerID. ok is false when the queue is idle.
func (s *Service) ClaimOne(ctx context.Context, workerID string, lease time.Duration) (Claim, bool, error) {
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	row, err := q.LockClaimableCollectionCycle(ctx, sqlcgen.LockClaimableCollectionCycleParams{QueuedBefore: now, RunningBefore: sql.NullTime{Time: now, Valid: true}})
	if errors.Is(err, sql.ErrNoRows) {
		return Claim{}, false, nil
	}
	if err != nil {
		return Claim{}, false, err
	}
	cycle, err := cycleFromRow(&row)
	if err != nil {
		return Claim{}, false, err
	}
	attempt := cycle.AttemptCount + 1
	if attempt > MaxAttempts {
		if _, err := q.FailActiveCollectionCycle(ctx, sqlcgen.FailActiveCollectionCycleParams{
			PortfolioID: row.PortfolioID, FinishedAt: sql.NullTime{Time: now, Valid: true},
			ErrorClass: nullable("max_attempts"), ErrorDetail: nullable("The cycle exceeded its retry budget."),
		}); err != nil {
			return Claim{}, false, err
		}
		return Claim{}, false, tx.Commit()
	}
	if _, err := q.MarkCollectionCycleRunning(ctx, sqlcgen.MarkCollectionCycleRunningParams{
		AttemptCount:   attempt,
		LeaseOwner:     sql.NullString{String: workerID, Valid: true},
		LeaseExpiresAt: sql.NullTime{Time: now.Add(lease), Valid: true},
		StartedAt:      sql.NullTime{Time: now, Valid: true},
		PortfolioID:    row.PortfolioID,
		ID:             row.ID,
	}); err != nil {
		return Claim{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Claim{}, false, err
	}
	cycle.Status = StatusRunning
	cycle.AttemptCount = attempt
	if cycle.StartedAt == nil {
		cycle.StartedAt = &now
	}
	return Claim{Cycle: cycle, WorkerID: workerID}, true, nil
}

// RenewLease extends a running cycle lease.
func (s *Service) RenewLease(ctx context.Context, claim *Claim, lease time.Duration) error {
	now := s.now().UTC()
	n, err := s.q.RenewCollectionCycleLease(ctx, sqlcgen.RenewCollectionCycleLeaseParams{
		LeaseExpiresAt: sql.NullTime{Time: now.Add(lease), Valid: true},
		PortfolioID:    claim.PortfolioID[:],
		ID:             claim.ID[:],
		LeaseOwner:     sql.NullString{String: claim.WorkerID, Valid: true},
		LeaseAt:        sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLostLease
	}
	return nil
}

// LockLease holds the claimed lease for the rest of q's transaction.
func LockLease(ctx context.Context, q *sqlcgen.Queries, claim *Claim, now time.Time) error {
	_, err := q.LockRunningCollectionCycle(ctx, sqlcgen.LockRunningCollectionCycleParams{
		PortfolioID: claim.PortfolioID[:], ID: claim.ID[:],
		LeaseOwner: sql.NullString{String: claim.WorkerID, Valid: true}, LeaseAt: sql.NullTime{Time: now, Valid: true},
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLostLease
	}
	return err
}

// MarkSucceeded completes the claimed cycle inside q's transaction.
func MarkSucceeded(ctx context.Context, q *sqlcgen.Queries, claim *Claim, now time.Time) error {
	n, err := q.MarkCollectionCycleSucceeded(ctx, sqlcgen.MarkCollectionCycleSucceededParams{
		FinishedAt:  sql.NullTime{Time: now, Valid: true},
		PortfolioID: claim.PortfolioID[:], ID: claim.ID[:],
		LeaseOwner: sql.NullString{String: claim.WorkerID, Valid: true}, LeaseAt: sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrLostLease
	}
	return nil
}

// Finish records the terminal or retry state of a claimed cycle.
func (s *Service) Finish(ctx context.Context, claim *Claim, result Result) error {
	now := s.now().UTC()
	if result.OK {
		return MarkSucceeded(ctx, s.q, claim, now)
	}
	owner := sql.NullString{String: claim.WorkerID, Valid: true}
	leaseAt := sql.NullTime{Time: now, Valid: true}
	var n int64
	var err error
	if result.Retry && claim.AttemptCount < MaxAttempts {
		retryAt := result.RetryAt
		if retryAt.IsZero() {
			retryAt = now
		}
		n, err = s.q.RequeueCollectionCycle(ctx, sqlcgen.RequeueCollectionCycleParams{
			AvailableAt: retryAt.UTC(), ErrorClass: nullable(result.ErrorClass), ErrorDetail: nullable(result.ErrorDetail),
			PortfolioID: claim.PortfolioID[:], ID: claim.ID[:], LeaseOwner: owner, LeaseAt: leaseAt,
		})
	} else {
		n, err = s.q.MarkCollectionCycleFailed(ctx, sqlcgen.MarkCollectionCycleFailedParams{
			FinishedAt: sql.NullTime{Time: now, Valid: true}, ErrorClass: nullable(result.ErrorClass), ErrorDetail: nullable(result.ErrorDetail),
			PortfolioID: claim.PortfolioID[:], ID: claim.ID[:], LeaseOwner: owner, LeaseAt: leaseAt,
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
