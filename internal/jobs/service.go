package jobs

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	minLimit     = 1
	maxLimit     = 200
	defaultLimit = 50
)

// Service owns durable job enqueueing and reads.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a job service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// Job is a durable unit of work.
type Job struct {
	ID              uuid.UUID
	PortfolioID     uuid.UUID
	Kind            string
	Status          string
	AttemptCount    uint32
	MaxAttempts     uint32
	LastErrorCode   *string
	LastErrorDetail *string
	CreatedAt       time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	Attempts        []Attempt
}

// Attempt is a recorded execution of a job.
type Attempt struct {
	AttemptNumber uint32
	WorkerID      string
	StartedAt     time.Time
	FinishedAt    *time.Time
	Outcome       string
	ErrorClass    *string
	ErrorCode     *string
	SafeDetail    *string
}

// Page is a cursor page of jobs.
type Page struct {
	Items      []Job
	Limit      int
	NextCursor *string
	HasMore    bool
}

// ClaimedJob is a leased job ready for execution.
type ClaimedJob struct {
	Job
	AttemptNumber uint32
	WorkerID      string
	StartedAt     time.Time
}

// EnqueueRebuild inserts or returns the active full-projection job for a Portfolio.
func (s *Service) EnqueueRebuild(ctx context.Context, portfolioID uuid.UUID, requestedBy *uuid.UUID, reason string) (Job, error) {
	if _, err := s.q.GetPortfolioByID(ctx, mustBytes(portfolioID)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	if existing, err := s.activeRebuild(ctx, portfolioID); err == nil {
		return existing, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Job{}, err
	}

	now := s.now().UTC()
	jobID, err := uuid.NewV7()
	if err != nil {
		return Job{}, err
	}
	if reason == "" {
		reason = "projection rebuild"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	params := sqlcgen.InsertJobParams{
		ID:           mustBytes(jobID),
		PortfolioID:  mustBytes(portfolioID),
		Kind:         KindProjection,
		Status:       statusQueued,
		Priority:     defaultPriority,
		DedupeKey:    dedupeFullRebuild,
		AvailableAt:  now,
		AttemptCount: 0,
		MaxAttempts:  defaultMaxAttempts,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if requestedBy != nil {
		params.RequestedByUserID = mustBytes(*requestedBy)
	}
	if err := q.InsertJob(ctx, params); err != nil {
		if mysql.IsDuplicate(err) {
			if existing, getErr := s.activeRebuild(ctx, portfolioID); getErr == nil {
				return existing, nil
			} else if getErr != nil {
				return Job{}, getErr
			}
		}
		return Job{}, err
	}
	if err := q.InsertProjectionJobDetail(ctx, sqlcgen.InsertProjectionJobDetailParams{
		PortfolioID: mustBytes(portfolioID),
		JobID:       mustBytes(jobID),
		Reason:      reason,
	}); err != nil {
		return Job{}, err
	}
	for _, kind := range FullProjectionKinds {
		if err := q.InsertProjectionJobKind(ctx, sqlcgen.InsertProjectionJobKindParams{
			PortfolioID:    mustBytes(portfolioID),
			JobID:          mustBytes(jobID),
			ProjectionKind: kind,
		}); err != nil {
			return Job{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	return s.Get(ctx, portfolioID, jobID)
}

func (s *Service) activeRebuild(ctx context.Context, portfolioID uuid.UUID) (Job, error) {
	row, err := s.q.GetActiveJobByDedupe(ctx, sqlcgen.GetActiveJobByDedupeParams{
		PortfolioID: mustBytes(portfolioID),
		DedupeKey:   sql.NullString{String: dedupeFullRebuild, Valid: true},
	})
	if err != nil {
		return Job{}, err
	}
	return jobFromFields(row.ID, row.PortfolioID, row.Kind, row.Status, row.AttemptCount, row.MaxAttempts, row.LastErrorCode, row.LastErrorDetail, row.CreatedAt, row.StartedAt, row.FinishedAt)
}

// List returns jobs for a Portfolio using authenticated cursor pagination.
func (s *Service) List(ctx context.Context, portfolioID uuid.UUID, limit int, cursor string) (Page, error) {
	if _, err := s.q.GetPortfolioByID(ctx, mustBytes(portfolioID)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Page{}, ErrNotFound
		}
		return Page{}, err
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit {
		return Page{}, ErrInvalidLimit
	}

	params := sqlcgen.ListJobsByPortfolioParams{
		PortfolioID:     mustBytes(portfolioID),
		HasCursor:       0,
		CursorCreatedAt: time.Time{},
		CursorID:        make([]byte, 16),
		Limit:           int32(limit + 1),
	}
	if cursor != "" {
		decoded, err := decodeCursor(cursor, portfolioID)
		if err != nil {
			return Page{}, err
		}
		params.HasCursor = 1
		params.CursorCreatedAt = time.UnixMicro(decoded.CreatedAt).UTC()
		params.CursorID = mustBytes(decoded.ID)
	}

	rows, err := s.q.ListJobsByPortfolio(ctx, params)
	if err != nil {
		return Page{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]Job, 0, len(rows))
	for i := range rows {
		item, err := jobFromFields(rows[i].ID, rows[i].PortfolioID, rows[i].Kind, rows[i].Status, rows[i].AttemptCount, rows[i].MaxAttempts, rows[i].LastErrorCode, rows[i].LastErrorDetail, rows[i].CreatedAt, rows[i].StartedAt, rows[i].FinishedAt)
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	page := Page{Items: items, Limit: limit, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := encodeCursor(listCursor{
			PortfolioID: portfolioID,
			CreatedAt:   last.CreatedAt.UTC().UnixMicro(),
			ID:          last.ID,
		})
		if err != nil {
			return Page{}, err
		}
		page.NextCursor = &encoded
	}
	return page, nil
}

// Get returns one job and its attempts.
func (s *Service) Get(ctx context.Context, portfolioID, jobID uuid.UUID) (Job, error) {
	row, err := s.q.GetJob(ctx, sqlcgen.GetJobParams{
		PortfolioID: mustBytes(portfolioID),
		ID:          mustBytes(jobID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	item, err := jobFromFields(row.ID, row.PortfolioID, row.Kind, row.Status, row.AttemptCount, row.MaxAttempts, row.LastErrorCode, row.LastErrorDetail, row.CreatedAt, row.StartedAt, row.FinishedAt)
	if err != nil {
		return Job{}, err
	}
	attempts, err := s.q.ListJobAttempts(ctx, sqlcgen.ListJobAttemptsParams{
		PortfolioID: mustBytes(portfolioID),
		JobID:       mustBytes(jobID),
	})
	if err != nil {
		return Job{}, err
	}
	item.Attempts = make([]Attempt, 0, len(attempts))
	for i := range attempts {
		item.Attempts = append(item.Attempts, attemptFromRow(&attempts[i]))
	}
	return item, nil
}

// ActiveGenerationID returns the Portfolio's selected generation, or uuid.Nil when unset.
func (s *Service) ActiveGenerationID(ctx context.Context, portfolioID uuid.UUID) (uuid.UUID, error) {
	raw, err := s.q.GetPortfolioGenerationPointer(ctx, mustBytes(portfolioID))
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	if len(raw) == 0 {
		return uuid.Nil, nil
	}
	id, err := uuid.FromBytes(raw)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ClaimOne leases one available job for workerID. ok is false when the queue is idle.
func (s *Service) ClaimOne(ctx context.Context, workerID string, lease time.Duration) (ClaimedJob, bool, error) {
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ClaimedJob{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	row, err := q.LockClaimableJob(ctx, sqlcgen.LockClaimableJobParams{
		QueuedBefore:  now,
		RunningBefore: sql.NullTime{Time: now, Valid: true},
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ClaimedJob{}, false, nil
	}
	if err != nil {
		return ClaimedJob{}, false, err
	}
	job, err := jobFromFields(row.ID, row.PortfolioID, row.Kind, row.Status, row.AttemptCount, row.MaxAttempts, row.LastErrorCode, row.LastErrorDetail, row.CreatedAt, row.StartedAt, row.FinishedAt)
	if err != nil {
		return ClaimedJob{}, false, err
	}
	attempt := job.AttemptCount + 1
	if attempt > job.MaxAttempts {
		if _, err := q.MarkJobFailed(ctx, sqlcgen.MarkJobFailedParams{
			LastErrorCode:   ptrToNull("max_attempts"),
			LastErrorDetail: ptrToNull("The job exceeded its retry budget."),
			FinishedAt:      sql.NullTime{Time: now, Valid: true},
			UpdatedAt:       now,
			ID:              mustBytes(job.ID),
			PortfolioID:     mustBytes(job.PortfolioID),
			LeaseOwner:      sql.NullString{String: stringFromNull(row.LeaseOwner), Valid: row.LeaseOwner.Valid},
		}); err != nil {
			return ClaimedJob{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return ClaimedJob{}, false, err
		}
		return ClaimedJob{}, false, nil
	}

	started := now
	if _, err := q.MarkJobRunning(ctx, sqlcgen.MarkJobRunningParams{
		AttemptCount:   attempt,
		LeaseOwner:     sql.NullString{String: workerID, Valid: true},
		LeaseExpiresAt: sql.NullTime{Time: now.Add(lease), Valid: true},
		StartedAt:      sql.NullTime{Time: started, Valid: true},
		UpdatedAt:      now,
		ID:             mustBytes(job.ID),
		PortfolioID:    mustBytes(job.PortfolioID),
	}); err != nil {
		return ClaimedJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ClaimedJob{}, false, err
	}
	job.Status = statusRunning
	job.AttemptCount = attempt
	job.StartedAt = &started
	return ClaimedJob{Job: job, AttemptNumber: attempt, WorkerID: workerID, StartedAt: started}, true, nil
}

// RenewLease extends a running job lease.
func (s *Service) RenewLease(ctx context.Context, job *ClaimedJob, lease time.Duration) error {
	now := s.now().UTC()
	n, err := s.q.RenewJobLease(ctx, sqlcgen.RenewJobLeaseParams{
		LeaseExpiresAt: sql.NullTime{Time: now.Add(lease), Valid: true},
		UpdatedAt:      now,
		ID:             mustBytes(job.ID),
		PortfolioID:    mustBytes(job.PortfolioID),
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

// FinishAttempt records the attempt and terminal or retry state.
func (s *Service) FinishAttempt(ctx context.Context, job *ClaimedJob, result Result) error {
	now := s.now().UTC()
	attemptID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	started := job.StartedAt
	if started.IsZero() {
		started = now
	}
	attemptParams := sqlcgen.InsertJobAttemptParams{
		ID:            mustBytes(attemptID),
		PortfolioID:   mustBytes(job.PortfolioID),
		JobID:         mustBytes(job.ID),
		AttemptNumber: job.AttemptNumber,
		WorkerID:      job.WorkerID,
		StartedAt:     started,
		FinishedAt:    sql.NullTime{Time: now, Valid: true},
		Outcome:       outcomeSucceeded,
		SafeDetail:    ptrToNull(result.SafeDetail),
	}
	if !result.OK {
		attemptParams.Outcome = outcomeFailed
		attemptParams.ErrorClass = ptrToNull(result.ErrorClass)
		attemptParams.ErrorCode = ptrToNull(result.ErrorCode)
		if result.RetryAt != nil {
			attemptParams.RetryAt = sql.NullTime{Time: result.RetryAt.UTC(), Valid: true}
		}
	}
	if err := q.InsertJobAttempt(ctx, attemptParams); err != nil {
		return err
	}

	switch {
	case result.OK:
		if _, err := q.MarkJobSucceeded(ctx, sqlcgen.MarkJobSucceededParams{
			FinishedAt:  sql.NullTime{Time: now, Valid: true},
			UpdatedAt:   now,
			ID:          mustBytes(job.ID),
			PortfolioID: mustBytes(job.PortfolioID),
			LeaseOwner:  sql.NullString{String: job.WorkerID, Valid: true},
		}); err != nil {
			return err
		}
	case result.Retry && job.AttemptNumber < job.MaxAttempts && result.RetryAt != nil:
		if _, err := q.RequeueJob(ctx, sqlcgen.RequeueJobParams{
			AvailableAt:     result.RetryAt.UTC(),
			LastErrorCode:   ptrToNull(result.ErrorCode),
			LastErrorDetail: ptrToNull(result.SafeDetail),
			UpdatedAt:       now,
			ID:              mustBytes(job.ID),
			PortfolioID:     mustBytes(job.PortfolioID),
			LeaseOwner:      sql.NullString{String: job.WorkerID, Valid: true},
		}); err != nil {
			return err
		}
	default:
		if _, err := q.MarkJobFailed(ctx, sqlcgen.MarkJobFailedParams{
			LastErrorCode:   ptrToNull(result.ErrorCode),
			LastErrorDetail: ptrToNull(result.SafeDetail),
			FinishedAt:      sql.NullTime{Time: now, Valid: true},
			UpdatedAt:       now,
			ID:              mustBytes(job.ID),
			PortfolioID:     mustBytes(job.PortfolioID),
			LeaseOwner:      sql.NullString{String: job.WorkerID, Valid: true},
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Result is the handler outcome for one attempt.
type Result struct {
	OK         bool
	Retry      bool
	RetryAt    *time.Time
	ErrorClass string
	ErrorCode  string
	SafeDetail string
}

func jobFromFields(id, portfolioID []byte, kind, status string, attemptCount, maxAttempts uint32, lastCode, lastDetail sql.NullString, createdAt time.Time, startedAt, finishedAt sql.NullTime) (Job, error) {
	parsedID, err := uuid.FromBytes(id)
	if err != nil {
		return Job{}, err
	}
	parsedPortfolio, err := uuid.FromBytes(portfolioID)
	if err != nil {
		return Job{}, err
	}
	item := Job{
		ID:              parsedID,
		PortfolioID:     parsedPortfolio,
		Kind:            kind,
		Status:          status,
		AttemptCount:    attemptCount,
		MaxAttempts:     maxAttempts,
		CreatedAt:       createdAt,
		LastErrorCode:   nullStringPtr(lastCode),
		LastErrorDetail: nullStringPtr(lastDetail),
	}
	if startedAt.Valid {
		t := startedAt.Time
		item.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		item.FinishedAt = &t
	}
	return item, nil
}

func attemptFromRow(row *sqlcgen.JobAttempt) Attempt {
	item := Attempt{
		AttemptNumber: row.AttemptNumber,
		WorkerID:      row.WorkerID,
		StartedAt:     row.StartedAt,
		Outcome:       row.Outcome,
		ErrorClass:    nullStringPtr(row.ErrorClass),
		ErrorCode:     nullStringPtr(row.ErrorCode),
		SafeDetail:    nullStringPtr(row.SafeDetail),
	}
	if row.FinishedAt.Valid {
		t := row.FinishedAt.Time
		item.FinishedAt = &t
	}
	return item
}

func mustBytes(id uuid.UUID) []byte {
	b, err := id.MarshalBinary()
	if err != nil {
		return id[:]
	}
	return b
}

func ptrToNull(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func stringFromNull(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}
