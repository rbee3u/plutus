// Package cycles owns collection cycles: the durable queue, worker leases, and
// cycle history reads.
package cycles

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/cursor"
)

// Cycle statuses and triggers.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"

	TriggerSchedule = "schedule"
	TriggerUser     = "user"

	// MaxAttempts bounds how many times one cycle is claimed before it fails.
	MaxAttempts = uint32(3)

	minLimit     = 1
	maxLimit     = 200
	defaultLimit = 50
)

// Service owns cycle enqueueing, reads, and leases.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a cycle service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// Point is the valuation point written by a succeeded collection cycle.
type Point struct {
	Revision               string
	CalculationVersion     uint64
	EvaluatedAt            time.Time
	TotalUSD               string
	AccountCount           int
	StaleAccountCount      int
	IncompleteAccountCount int
	DetailState            string
}

// Cycle is durable financial history with its point when one exists.
// ErrorClass and ErrorDetail describe the latest failed attempt.
type Cycle struct {
	ID, PortfolioID         uuid.UUID
	Status, Trigger         string
	AttemptCount            uint32
	ErrorClass, ErrorDetail *string
	CreatedAt               time.Time
	StartedAt, FinishedAt   *time.Time
	Point                   *Point
}

// Active reports whether the cycle is queued or running.
func (c *Cycle) Active() bool {
	return c.Status == StatusQueued || c.Status == StatusRunning
}

// Page is a cursor page of cycles.
type Page struct {
	Items      []Cycle
	Limit      int
	NextCursor *string
	HasMore    bool
}

// Enqueue returns the portfolio's active cycle, or creates a queued one.
func (s *Service) Enqueue(ctx context.Context, portfolioID uuid.UUID, trigger string) (Cycle, bool, error) {
	type result struct {
		cycle   Cycle
		created bool
	}
	value, err := mysql.WithTx(ctx, s.db, func(q *sqlcgen.Queries) (result, error) {
		// Serialize first insertion as well as active-cycle reuse within the portfolio.
		if _, err := q.LockPortfolio(ctx, portfolioID[:]); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return result{}, ErrNotFound
			}
			return result{}, err
		}
		existing, err := q.LockActiveCollectionCycle(ctx, portfolioID[:])
		if err == nil {
			cycle, err := cycleFromRow(&existing)
			return result{cycle: cycle}, err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return result{}, err
		}
		id, err := uuid.NewV7()
		if err != nil {
			return result{}, err
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		if err := q.InsertCollectionCycle(ctx, sqlcgen.InsertCollectionCycleParams{PortfolioID: portfolioID[:], ID: id[:], TriggerKind: trigger, CreatedAt: now}); err != nil {
			return result{}, err
		}
		row, err := q.GetCollectionCycle(ctx, sqlcgen.GetCollectionCycleParams{PortfolioID: portfolioID[:], ID: id[:]})
		if err != nil {
			return result{}, err
		}
		cycle, err := cycleFromRow(&row)
		return result{cycle: cycle, created: true}, err
	})
	return value.cycle, value.created, err
}

// Get reads one cycle without its point.
func (s *Service) Get(ctx context.Context, portfolioID, cycleID uuid.UUID) (Cycle, error) {
	row, err := mysql.Queries(ctx, s.db).GetCollectionCycle(ctx, sqlcgen.GetCollectionCycleParams{PortfolioID: portfolioID[:], ID: cycleID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Cycle{}, ErrNotFound
	}
	if err != nil {
		return Cycle{}, err
	}
	return cycleFromRow(&row)
}

// List returns cycles newest first with their valuation points.
func (s *Service) List(ctx context.Context, portfolioID uuid.UUID, limit int, rawCursor string) (Page, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit {
		return Page{}, ErrInvalidLimit
	}
	params := sqlcgen.ListCyclesWithPointsParams{
		PortfolioID:     portfolioID[:],
		HasCursor:       0,
		CursorCreatedAt: time.Time{},
		CursorID:        make([]byte, 16),
		Limit:           int32(limit + 1),
	}
	if rawCursor != "" {
		decoded, err := decodeCursor(rawCursor, portfolioID)
		if err != nil {
			return Page{}, err
		}
		params.HasCursor = 1
		params.CursorCreatedAt = time.UnixMicro(decoded.TimeMicro).UTC()
		params.CursorID = decoded.ID[:]
	}
	rows, err := s.q.ListCyclesWithPoints(ctx, params)
	if err != nil {
		return Page{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]Cycle, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		id, err := uuid.FromBytes(r.ID)
		if err != nil {
			return Page{}, err
		}
		item := Cycle{
			ID: id, PortfolioID: portfolioID, Status: r.Status, Trigger: r.TriggerKind, AttemptCount: r.AttemptCount, CreatedAt: r.CreatedAt,
			ErrorClass: nullStringPtr(r.ErrorClass), ErrorDetail: nullStringPtr(r.ErrorDetail),
			StartedAt: nullTimePtr(r.StartedAt), FinishedAt: nullTimePtr(r.FinishedAt),
		}
		if len(r.PointCycleID) == 16 && r.EvaluatedAt.Valid {
			if r.CalculationVersion.Int64 < 1 {
				return Page{}, errors.New("invalid calculation version")
			}
			point := &Point{
				Revision: id.String(), CalculationVersion: uint64(r.CalculationVersion.Int64),
				EvaluatedAt:            r.EvaluatedAt.Time,
				TotalUSD:               r.TotalUsd.String,
				AccountCount:           int(r.AccountCount.Int32),
				StaleAccountCount:      int(r.StaleAccountCount.Int32),
				IncompleteAccountCount: int(r.IncompleteAccountCount.Int32),
				DetailState:            r.DetailState.String,
			}
			if point.DetailState == "pruning" {
				point.DetailState = "expired"
			}
			item.Point = point
		}
		items = append(items, item)
	}
	page := Page{Items: items, Limit: limit, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := cursorCodec.Encode(cursor.Cursor{PortfolioID: portfolioID, TimeMicro: last.CreatedAt.UTC().UnixMicro(), ID: last.ID})
		if err != nil {
			return Page{}, err
		}
		page.NextCursor = &encoded
	}
	return page, nil
}

// SchedulingRow is one portfolio the scheduler may need to enqueue.
type SchedulingRow struct {
	PortfolioID uuid.UUID
	Interval    time.Duration
	LastCycleAt *time.Time
}

// ListForScheduling returns portfolios with at least one enabled connection.
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
	var raw string
	switch v := value.(type) {
	case time.Time:
		return v, true
	case []byte:
		raw = string(v)
	case string:
		raw = v
	default:
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999", raw)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func cycleFromRow(row *sqlcgen.CollectionCycle) (Cycle, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Cycle{}, err
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Cycle{}, err
	}
	return Cycle{
		ID: id, PortfolioID: portfolioID, Status: row.Status, Trigger: row.TriggerKind, AttemptCount: row.AttemptCount, CreatedAt: row.CreatedAt,
		ErrorClass: nullStringPtr(row.ErrorClass), ErrorDetail: nullStringPtr(row.ErrorDetail),
		StartedAt: nullTimePtr(row.StartedAt), FinishedAt: nullTimePtr(row.FinishedAt),
	}, nil
}

func nullable(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return new(v.String)
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return new(v.Time)
}
