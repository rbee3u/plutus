// Package jobs owns the durable job queue: Collection Cycles and valuation recalculations.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
)

// Job kinds, statuses, and triggers.
const (
	KindCollectionCycle = "collection_cycle"
	KindValuationRecalc = "valuation_recalc"

	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"

	TriggerSchedule = "schedule"
	TriggerManual   = "manual"
	TriggerSystem   = "system"

	minLimit           = 1
	maxLimit           = 200
	defaultLimit       = 50
	defaultMaxAttempts = uint32(3)
	dedupeCycle        = "cycle"
	dedupeRecalc       = "recalc"
)

// Service owns durable job enqueueing, reads, and leases.
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
	ID             uuid.UUID
	PortfolioID    uuid.UUID
	Kind           string
	Status         string
	Trigger        string
	Payload        json.RawMessage
	AttemptCount   uint32
	MaxAttempts    uint32
	LastErrorClass *string
	// LastDetail is the outcome of the last attempt: a short summary on success, the sanitized error otherwise.
	LastDetail  *string
	CreatedAt   time.Time
	AvailableAt time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
}

// Active reports whether the job is queued or running.
func (j *Job) Active() bool {
	return j.Status == StatusQueued || j.Status == StatusRunning
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
	WorkerID string
}

// RecalcPayload is the payload of a valuation_recalc job.
type RecalcPayload struct {
	Reason    string `json:"reason"`
	ListingID string `json:"listingId,omitempty"`
}

// EnqueueCycle inserts one Collection Cycle for a Portfolio or returns the active one.
func (s *Service) EnqueueCycle(ctx context.Context, portfolioID uuid.UUID, trigger string, requestedBy *uuid.UUID) (Job, bool, error) {
	return s.enqueue(ctx, portfolioID, KindCollectionCycle, trigger, dedupeCycle, nil, requestedBy)
}

// EnqueueRecalc inserts one valuation recalculation for a Portfolio or returns the active one.
func (s *Service) EnqueueRecalc(ctx context.Context, portfolioID uuid.UUID, payload RecalcPayload, requestedBy *uuid.UUID) (Job, bool, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Job{}, false, err
	}
	return s.enqueue(ctx, portfolioID, KindValuationRecalc, TriggerSystem, dedupeRecalc, raw, requestedBy)
}

func (s *Service) enqueue(ctx context.Context, portfolioID uuid.UUID, kind, trigger, dedupe string, payload []byte, requestedBy *uuid.UUID) (Job, bool, error) {
	if _, err := s.q.GetPortfolioByID(ctx, portfolioID[:]); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, false, ErrNotFound
		}
		return Job{}, false, err
	}
	if existing, ok, err := s.active(ctx, portfolioID, dedupe); err != nil {
		return Job{}, false, err
	} else if ok {
		return existing, false, nil
	}
	now := s.now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return Job{}, false, err
	}
	params := sqlcgen.InsertJobParams{
		ID:          id[:],
		PortfolioID: portfolioID[:],
		Kind:        kind,
		TriggerKind: trigger,
		DedupeKey:   dedupe,
		AvailableAt: now,
		MaxAttempts: defaultMaxAttempts,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if len(payload) > 0 {
		params.PayloadJson = sqljson.Column(payload)
	}
	if requestedBy != nil {
		params.RequestedByUserID = requestedBy[:]
	}
	if err := s.q.InsertJob(ctx, params); err != nil {
		if mysql.IsDuplicate(err) {
			if existing, ok, getErr := s.active(ctx, portfolioID, dedupe); getErr == nil && ok {
				return existing, false, nil
			}
		}
		return Job{}, false, err
	}
	job, err := s.Get(ctx, portfolioID, id)
	return job, true, err
}

func (s *Service) active(ctx context.Context, portfolioID uuid.UUID, dedupe string) (Job, bool, error) {
	row, err := s.q.GetActiveJobByDedupe(ctx, sqlcgen.GetActiveJobByDedupeParams{
		PortfolioID: portfolioID[:],
		DedupeKey:   sql.NullString{String: dedupe, Valid: true},
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	job, err := fromFields(row.ID, row.PortfolioID, row.Kind, row.Status, row.Trigger, row.PayloadJson, row.AttemptCount, row.MaxAttempts, row.LastErrorClass, row.LastDetail, row.CreatedAt, row.AvailableAt, row.StartedAt, row.FinishedAt)
	return job, err == nil, err
}

// List returns jobs for a Portfolio, newest first, using authenticated cursor pagination.
func (s *Service) List(ctx context.Context, portfolioID uuid.UUID, kind string, limit int, cursor string) (Page, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit {
		return Page{}, ErrInvalidLimit
	}
	params := sqlcgen.ListJobsByPortfolioParams{
		PortfolioID:     portfolioID[:],
		FilterKind:      flag(kind != ""),
		Kind:            kind,
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
		params.CursorID = decoded.ID[:]
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
		r := &rows[i]
		item, err := fromFields(r.ID, r.PortfolioID, r.Kind, r.Status, r.Trigger, r.PayloadJson, r.AttemptCount, r.MaxAttempts, r.LastErrorClass, r.LastDetail, r.CreatedAt, r.AvailableAt, r.StartedAt, r.FinishedAt)
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	page := Page{Items: items, Limit: limit, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := encodeCursor(listCursor{PortfolioID: portfolioID, CreatedAt: last.CreatedAt.UTC().UnixMicro(), ID: last.ID})
		if err != nil {
			return Page{}, err
		}
		page.NextCursor = &encoded
	}
	return page, nil
}

// Get returns one job.
func (s *Service) Get(ctx context.Context, portfolioID, jobID uuid.UUID) (Job, error) {
	row, err := s.q.GetJob(ctx, sqlcgen.GetJobParams{PortfolioID: portfolioID[:], ID: jobID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	return fromFields(row.ID, row.PortfolioID, row.Kind, row.Status, row.Trigger, row.PayloadJson, row.AttemptCount, row.MaxAttempts, row.LastErrorClass, row.LastDetail, row.CreatedAt, row.AvailableAt, row.StartedAt, row.FinishedAt)
}

// Result is the handler outcome for one attempt.
type Result struct {
	OK         bool
	Retry      bool
	RetryAt    time.Time
	ErrorClass string
	// Detail is stored as the job's LastDetail: a summary on success, the sanitized error otherwise.
	Detail string
}

// SchedulingRow is one Portfolio the scheduler may need to enqueue.
type SchedulingRow struct {
	PortfolioID uuid.UUID
	Interval    time.Duration
	LastCycleAt *time.Time
}

// ListForScheduling returns Portfolios with at least one enabled Connection.
func (s *Service) ListForScheduling(ctx context.Context) ([]SchedulingRow, error) {
	rows, err := s.q.ListPortfoliosForScheduling(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SchedulingRow, 0, len(rows))
	for i := range rows {
		id, err := uuid.FromBytes(rows[i].ID)
		if err != nil {
			return nil, err
		}
		item := SchedulingRow{PortfolioID: id, Interval: time.Duration(rows[i].CollectionIntervalSeconds) * time.Second}
		if t, ok := asTime(rows[i].LastCycleAt); ok {
			item.LastCycleAt = &t
		}
		out = append(out, item)
	}
	return out, nil
}

func asTime(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		return v, true
	case []byte:
		t, err := time.Parse("2006-01-02 15:04:05.999999", string(v))
		if err != nil {
			return time.Time{}, false
		}
		return t.UTC(), true
	case string:
		t, err := time.Parse("2006-01-02 15:04:05.999999", v)
		if err != nil {
			return time.Time{}, false
		}
		return t.UTC(), true
	default:
		return time.Time{}, false
	}
}

func fromFields(id, portfolioID []byte, kind, status, trigger string, payload sqljson.Column, attemptCount, maxAttempts uint32, lastClass, lastDetail sql.NullString, createdAt, availableAt time.Time, startedAt, finishedAt sql.NullTime) (Job, error) {
	parsedID, err := uuid.FromBytes(id)
	if err != nil {
		return Job{}, err
	}
	parsedPortfolio, err := uuid.FromBytes(portfolioID)
	if err != nil {
		return Job{}, err
	}
	item := Job{
		ID:             parsedID,
		PortfolioID:    parsedPortfolio,
		Kind:           kind,
		Status:         status,
		Trigger:        trigger,
		AttemptCount:   attemptCount,
		MaxAttempts:    maxAttempts,
		CreatedAt:      createdAt,
		AvailableAt:    availableAt,
		LastErrorClass: nullPtr(lastClass),
		LastDetail:     nullPtr(lastDetail),
	}
	if len(payload) > 0 {
		item.Payload = json.RawMessage(payload)
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

func nullable(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func flag(on bool) int {
	if on {
		return 1
	}
	return 0
}

// PruneFinished deletes finished jobs older than before that no Account Report or Valuation Point
// references. Collection Cycles that produced reports are kept indefinitely with their data.
func (s *Service) PruneFinished(ctx context.Context, before time.Time) (int64, error) {
	return s.q.DeleteFinishedJobsBefore(ctx, sql.NullTime{Time: before.UTC(), Valid: true})
}
