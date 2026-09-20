package capital

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
)

const (
	minLimit     = 1
	maxLimit     = 200
	defaultLimit = 50
)

var decimalPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// Service owns Capital Baseline and Capital Flow revisions.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a capital service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// Baseline is the current logical capital baseline.
type Baseline struct {
	ID                uuid.UUID `json:"id"`
	PortfolioID       uuid.UUID `json:"portfolioId"`
	CurrentRevisionID uuid.UUID `json:"currentRevisionId"`
	Version           uint64    `json:"version"`
	EffectiveAt       time.Time `json:"effectiveAt"`
	AssetID           uuid.UUID `json:"assetId"`
	Quantity          string    `json:"quantity"`
	Note              *string   `json:"note,omitempty"`
}

// BaselineRevision is an append-only baseline revision.
type BaselineRevision struct {
	ID              uuid.UUID
	EffectiveAt     time.Time
	AssetID         uuid.UUID
	Quantity        string
	Note            *string
	CreatedByUserID uuid.UUID
	CreatedAt       time.Time
}

// Flow is the current capital flow head plus current revision.
type Flow struct {
	ID                uuid.UUID  `json:"id"`
	PortfolioID       uuid.UUID  `json:"portfolioId"`
	CurrentRevisionID uuid.UUID  `json:"currentRevisionId"`
	Version           uint64     `json:"version"`
	Direction         string     `json:"direction"`
	Quantity          string     `json:"quantity"`
	AssetID           uuid.UUID  `json:"assetId"`
	EffectiveAt       time.Time  `json:"effectiveAt"`
	AccountID         *uuid.UUID `json:"accountId,omitempty"`
	Note              *string    `json:"note,omitempty"`
	State             string     `json:"state"`
}

// FlowRevision is an append-only flow revision.
type FlowRevision struct {
	ID              uuid.UUID
	Direction       string
	Quantity        string
	AssetID         uuid.UUID
	EffectiveAt     time.Time
	AccountID       *uuid.UUID
	Note            *string
	State           string
	CreatedByUserID uuid.UUID
	CreatedAt       time.Time
}

// RevisionPage is a cursor page of revisions.
type RevisionPage[T any] struct {
	Items      []T
	Limit      int
	NextCursor *string
	HasMore    bool
}

// FlowPage is an offset page of current flows.
type FlowPage struct {
	Items  []Flow
	Offset int
	Limit  int
	Total  int
}

// PutBaselineInput creates or corrects the logical baseline.
type PutBaselineInput struct {
	ActorUserID uuid.UUID
	RequestID   string
	EffectiveAt time.Time
	AssetID     uuid.UUID
	Quantity    string
	Note        *string
	IfMatch     string
}

// CreateFlowInput creates a capital flow.
type CreateFlowInput struct {
	ActorUserID uuid.UUID
	RequestID   string
	Direction   string
	Quantity    string
	AssetID     uuid.UUID
	EffectiveAt time.Time
	AccountID   *uuid.UUID
	Note        *string
}

// PatchFlowInput appends an edited revision.
type PatchFlowInput struct {
	ActorUserID  uuid.UUID
	RequestID    string
	Direction    *string
	Quantity     *string
	AssetID      *uuid.UUID
	EffectiveAt  *time.Time
	AccountID    *uuid.UUID
	ClearAccount bool
	Note         *string
}

// ETag returns the baseline validator.
func (b *Baseline) ETag() string {
	return fmt.Sprintf(`"capital-baseline:%d"`, b.Version)
}

// ETag returns the flow validator.
func (f *Flow) ETag() string {
	return fmt.Sprintf(`"capital-flow:%d"`, f.Version)
}

// ParseBaselineETag extracts the version or a create wildcard.
func ParseBaselineETag(value string) (uint64, bool, bool) {
	if value == "*" {
		return 0, true, true
	}
	var version uint64
	if _, err := fmt.Sscanf(value, `"capital-baseline:%d"`, &version); err != nil {
		return 0, false, false
	}
	return version, false, true
}

// ParseFlowETag extracts the flow version.
func ParseFlowETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"capital-flow:%d"`, &version); err != nil {
		return 0, false
	}
	return version, true
}

// GetBaseline returns the current baseline.
func (s *Service) GetBaseline(ctx context.Context, portfolioID uuid.UUID) (Baseline, error) {
	return s.getBaseline(ctx, s.q, portfolioID)
}

// PutBaseline creates the logical baseline or appends a correction.
func (s *Service) PutBaseline(ctx context.Context, portfolioID uuid.UUID, in *PutBaselineInput) (Baseline, bool, error) {
	if in == nil {
		return Baseline{}, false, ErrInvalid
	}
	quantity, err := normalizeDecimal(in.Quantity)
	if err != nil {
		return Baseline{}, false, err
	}
	if in.EffectiveAt.IsZero() {
		return Baseline{}, false, ErrInvalid
	}
	if _, err := s.q.GetAssetByID(ctx, mustBin(in.AssetID)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Baseline{}, false, ErrInvalid
		}
		return Baseline{}, false, err
	}
	version, star, ok := ParseBaselineETag(in.IfMatch)
	if !ok {
		return Baseline{}, false, ErrPrecondition
	}
	current, err := s.GetBaseline(ctx, portfolioID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Baseline{}, false, err
	}
	exists := err == nil
	if star && exists {
		return current, false, ErrExists
	}
	if !star && !exists {
		return Baseline{}, false, ErrNotFound
	}
	if exists && current.Version != version {
		return current, false, ErrPrecondition
	}

	revisionID, err := uuid.NewV7()
	if err != nil {
		return Baseline{}, false, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Baseline{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	created := !exists
	var baselineID uuid.UUID
	if created {
		baselineID, err = uuid.NewV7()
		if err != nil {
			return Baseline{}, false, err
		}
		if err := q.InsertCapitalBaseline(ctx, sqlcgen.InsertCapitalBaselineParams{
			ID:          mustBin(baselineID),
			PortfolioID: mustBin(portfolioID),
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			if mysql.IsDuplicate(err) {
				return Baseline{}, false, ErrExists
			}
			return Baseline{}, false, err
		}
	} else {
		baselineID = current.ID
	}
	if err := q.InsertCapitalBaselineRevision(ctx, sqlcgen.InsertCapitalBaselineRevisionParams{
		ID:                mustBin(revisionID),
		PortfolioID:       mustBin(portfolioID),
		CapitalBaselineID: mustBin(baselineID),
		EffectiveAt:       in.EffectiveAt.UTC(),
		AssetID:           mustBin(in.AssetID),
		Quantity:          quantity,
		Note:              nullNote(in.Note),
		CreatedByUserID:   mustBin(in.ActorUserID),
		CreatedAt:         now,
	}); err != nil {
		return Baseline{}, false, err
	}
	if created {
		if err := q.SetCapitalBaselineRevision(ctx, sqlcgen.SetCapitalBaselineRevisionParams{
			CurrentRevisionID: mustBin(revisionID),
			UpdatedAt:         now,
			PortfolioID:       mustBin(portfolioID),
			ID:                mustBin(baselineID),
		}); err != nil {
			return Baseline{}, false, err
		}
	} else {
		n, err := q.UpdateCapitalBaselinePointer(ctx, sqlcgen.UpdateCapitalBaselinePointerParams{
			CurrentRevisionID: mustBin(revisionID),
			UpdatedAt:         now,
			PortfolioID:       mustBin(portfolioID),
			ID:                mustBin(baselineID),
			Version:           version,
		})
		if err != nil {
			return Baseline{}, false, err
		}
		if n == 0 {
			latest, getErr := s.GetBaseline(ctx, portfolioID)
			if getErr != nil {
				return Baseline{}, false, getErr
			}
			return latest, false, ErrPrecondition
		}
	}
	after, err := s.getBaseline(ctx, q, portfolioID)
	if err != nil {
		return Baseline{}, false, err
	}
	action := "capital_baseline.correct"
	if created {
		action = "capital_baseline.create"
	}
	if err := insertAudit(ctx, q, portfolioID, in.ActorUserID, in.RequestID, action, "capital_baseline", baselineID, currentOrNil(exists, &current), after, now); err != nil {
		return Baseline{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Baseline{}, false, err
	}
	return after, created, nil
}

// ListBaselineRevisions returns cursor-paged baseline history.
func (s *Service) ListBaselineRevisions(ctx context.Context, portfolioID uuid.UUID, limit int, cursor string) (RevisionPage[BaselineRevision], error) {
	head, err := s.GetBaseline(ctx, portfolioID)
	if err != nil {
		return RevisionPage[BaselineRevision]{}, err
	}
	limit, err = normalizeLimit(limit)
	if err != nil {
		return RevisionPage[BaselineRevision]{}, err
	}
	params := sqlcgen.ListCapitalBaselineRevisionsParams{
		PortfolioID:       mustBin(portfolioID),
		CapitalBaselineID: mustBin(head.ID),
		HasCursor:         0,
		CursorID:          make([]byte, 16),
		Limit:             int32(limit + 1),
	}
	if cursor != "" {
		decoded, err := decodeCursor(cursor, portfolioID)
		if err != nil {
			return RevisionPage[BaselineRevision]{}, err
		}
		params.HasCursor = 1
		params.CursorCreatedAt = time.UnixMicro(decoded.CreatedAt).UTC()
		params.CursorID = mustBin(decoded.ID)
	}
	rows, err := s.q.ListCapitalBaselineRevisions(ctx, params)
	if err != nil {
		return RevisionPage[BaselineRevision]{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]BaselineRevision, 0, len(rows))
	for i := range rows {
		item, err := baselineRevisionFromRow(&rows[i])
		if err != nil {
			return RevisionPage[BaselineRevision]{}, err
		}
		items = append(items, item)
	}
	page := RevisionPage[BaselineRevision]{Items: items, Limit: limit, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := encodeCursor(listCursor{PortfolioID: portfolioID, CreatedAt: last.CreatedAt.UTC().UnixMicro(), ID: last.ID})
		if err != nil {
			return RevisionPage[BaselineRevision]{}, err
		}
		page.NextCursor = &encoded
	}
	return page, nil
}

func (s *Service) getBaseline(ctx context.Context, q *sqlcgen.Queries, portfolioID uuid.UUID) (Baseline, error) {
	head, err := q.GetCapitalBaselineByPortfolio(ctx, mustBin(portfolioID))
	if errors.Is(err, sql.ErrNoRows) {
		return Baseline{}, ErrNotFound
	}
	if err != nil {
		return Baseline{}, err
	}
	if len(head.CurrentRevisionID) != 16 {
		return Baseline{}, ErrNotFound
	}
	rev, err := q.GetCapitalBaselineRevision(ctx, sqlcgen.GetCapitalBaselineRevisionParams{
		PortfolioID: mustBin(portfolioID),
		ID:          head.CurrentRevisionID,
	})
	if err != nil {
		return Baseline{}, err
	}
	return baselineFrom(&head, &rev)
}

// ListFlows returns current active flows.
func (s *Service) ListFlows(ctx context.Context, portfolioID uuid.UUID, offset, limit int) (FlowPage, error) {
	offset, limit, err := normalizePage(offset, limit)
	if err != nil {
		return FlowPage{}, err
	}
	total, err := s.q.CountActiveCapitalFlows(ctx, mustBin(portfolioID))
	if err != nil {
		return FlowPage{}, err
	}
	rows, err := s.q.ListActiveCapitalFlows(ctx, sqlcgen.ListActiveCapitalFlowsParams{
		PortfolioID: mustBin(portfolioID),
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	if err != nil {
		return FlowPage{}, err
	}
	items := make([]Flow, 0, len(rows))
	for i := range rows {
		item, err := flowFromListRow(&rows[i])
		if err != nil {
			return FlowPage{}, err
		}
		items = append(items, item)
	}
	return FlowPage{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

// GetFlow returns one flow including a deleted current revision.
func (s *Service) GetFlow(ctx context.Context, portfolioID, flowID uuid.UUID) (Flow, error) {
	return s.getFlow(ctx, s.q, portfolioID, flowID)
}

// CreateFlow inserts a flow head and first active revision.
func (s *Service) CreateFlow(ctx context.Context, portfolioID uuid.UUID, in *CreateFlowInput) (Flow, error) {
	if in == nil {
		return Flow{}, ErrInvalid
	}
	draft, err := s.validateNewRevision(ctx, portfolioID, in.Direction, in.Quantity, in.AssetID, in.EffectiveAt, in.AccountID)
	if err != nil {
		return Flow{}, err
	}
	flowID, err := uuid.NewV7()
	if err != nil {
		return Flow{}, err
	}
	revisionID, err := uuid.NewV7()
	if err != nil {
		return Flow{}, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Flow{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := q.InsertCapitalFlow(ctx, sqlcgen.InsertCapitalFlowParams{
		ID:          mustBin(flowID),
		PortfolioID: mustBin(portfolioID),
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		return Flow{}, err
	}
	if err := q.InsertCapitalFlowRevision(ctx, flowRevisionParams(portfolioID, flowID, revisionID, &draft, in.Note, "active", in.ActorUserID, now)); err != nil {
		return Flow{}, err
	}
	if err := q.SetCapitalFlowRevision(ctx, sqlcgen.SetCapitalFlowRevisionParams{
		CurrentRevisionID: mustBin(revisionID),
		UpdatedAt:         now,
		PortfolioID:       mustBin(portfolioID),
		ID:                mustBin(flowID),
	}); err != nil {
		return Flow{}, err
	}
	item, err := s.getFlow(ctx, q, portfolioID, flowID)
	if err != nil {
		return Flow{}, err
	}
	if err := insertAudit(ctx, q, portfolioID, in.ActorUserID, in.RequestID, "capital_flow.create", "capital_flow", flowID, nil, item, now); err != nil {
		return Flow{}, err
	}
	if err := tx.Commit(); err != nil {
		return Flow{}, err
	}
	return item, nil
}

// PatchFlow appends an edited active revision.
func (s *Service) PatchFlow(ctx context.Context, portfolioID, flowID uuid.UUID, version uint64, in *PatchFlowInput) (Flow, error) {
	if in == nil || in.empty() {
		return Flow{}, ErrInvalid
	}
	current, err := s.GetFlow(ctx, portfolioID, flowID)
	if err != nil {
		return Flow{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	next := current
	if in.Direction != nil {
		next.Direction = *in.Direction
	}
	if in.Quantity != nil {
		next.Quantity = *in.Quantity
	}
	if in.AssetID != nil {
		next.AssetID = *in.AssetID
	}
	if in.EffectiveAt != nil {
		next.EffectiveAt = *in.EffectiveAt
	}
	if in.ClearAccount {
		next.AccountID = nil
	} else if in.AccountID != nil {
		next.AccountID = in.AccountID
	}
	if in.Note != nil {
		next.Note = in.Note
	}
	draft, err := s.validateNewRevision(ctx, portfolioID, next.Direction, next.Quantity, next.AssetID, next.EffectiveAt, next.AccountID)
	if err != nil {
		return Flow{}, err
	}
	return s.appendFlowRevision(ctx, &current, &draft, in.Note, "active", "capital_flow.correct", in.ActorUserID, in.RequestID, version)
}

// DeleteFlow appends a deleted revision while preserving history.
func (s *Service) DeleteFlow(ctx context.Context, portfolioID, flowID uuid.UUID, version uint64, actor uuid.UUID, requestID string) (Flow, error) {
	current, err := s.GetFlow(ctx, portfolioID, flowID)
	if err != nil {
		return Flow{}, err
	}
	if current.Version != version {
		return current, ErrPrecondition
	}
	draft := revisionDraft{Direction: current.Direction, Quantity: current.Quantity, AssetID: current.AssetID, EffectiveAt: current.EffectiveAt, AccountID: current.AccountID}
	return s.appendFlowRevision(ctx, &current, &draft, current.Note, "deleted", "capital_flow.delete", actor, requestID, version)
}

// ListFlowRevisions returns cursor-paged flow history.
func (s *Service) ListFlowRevisions(ctx context.Context, portfolioID, flowID uuid.UUID, limit int, cursor string) (RevisionPage[FlowRevision], error) {
	if _, err := s.GetFlow(ctx, portfolioID, flowID); err != nil {
		return RevisionPage[FlowRevision]{}, err
	}
	limit, err := normalizeLimit(limit)
	if err != nil {
		return RevisionPage[FlowRevision]{}, err
	}
	params := sqlcgen.ListCapitalFlowRevisionsParams{
		PortfolioID:   mustBin(portfolioID),
		CapitalFlowID: mustBin(flowID),
		HasCursor:     0,
		CursorID:      make([]byte, 16),
		Limit:         int32(limit + 1),
	}
	if cursor != "" {
		decoded, err := decodeCursor(cursor, portfolioID)
		if err != nil {
			return RevisionPage[FlowRevision]{}, err
		}
		params.HasCursor = 1
		params.CursorCreatedAt = time.UnixMicro(decoded.CreatedAt).UTC()
		params.CursorID = mustBin(decoded.ID)
	}
	rows, err := s.q.ListCapitalFlowRevisions(ctx, params)
	if err != nil {
		return RevisionPage[FlowRevision]{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]FlowRevision, 0, len(rows))
	for i := range rows {
		item, err := flowRevisionFromRow(&rows[i])
		if err != nil {
			return RevisionPage[FlowRevision]{}, err
		}
		items = append(items, item)
	}
	page := RevisionPage[FlowRevision]{Items: items, Limit: limit, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := encodeCursor(listCursor{PortfolioID: portfolioID, CreatedAt: last.CreatedAt.UTC().UnixMicro(), ID: last.ID})
		if err != nil {
			return RevisionPage[FlowRevision]{}, err
		}
		page.NextCursor = &encoded
	}
	return page, nil
}

type revisionDraft struct {
	Direction   string
	Quantity    string
	AssetID     uuid.UUID
	EffectiveAt time.Time
	AccountID   *uuid.UUID
}

func (s *Service) validateNewRevision(ctx context.Context, portfolioID uuid.UUID, direction, quantity string, assetID uuid.UUID, effectiveAt time.Time, accountID *uuid.UUID) (revisionDraft, error) {
	if direction != "contribution" && direction != "withdrawal" {
		return revisionDraft{}, ErrInvalid
	}
	qty, err := positiveDecimal(quantity)
	if err != nil {
		return revisionDraft{}, err
	}
	if effectiveAt.IsZero() {
		return revisionDraft{}, ErrInvalid
	}
	if _, err := s.q.GetAssetByID(ctx, mustBin(assetID)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return revisionDraft{}, ErrInvalid
		}
		return revisionDraft{}, err
	}
	if accountID != nil {
		if _, err := s.q.GetAccount(ctx, sqlcgen.GetAccountParams{PortfolioID: mustBin(portfolioID), ID: mustBin(*accountID)}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return revisionDraft{}, ErrInvalid
			}
			return revisionDraft{}, err
		}
	}
	return revisionDraft{Direction: direction, Quantity: qty, AssetID: assetID, EffectiveAt: effectiveAt.UTC(), AccountID: accountID}, nil
}

func (s *Service) appendFlowRevision(ctx context.Context, current *Flow, draft *revisionDraft, note *string, state, action string, actor uuid.UUID, requestID string, version uint64) (Flow, error) {
	revisionID, err := uuid.NewV7()
	if err != nil {
		return Flow{}, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Flow{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := q.InsertCapitalFlowRevision(ctx, flowRevisionParams(current.PortfolioID, current.ID, revisionID, draft, note, state, actor, now)); err != nil {
		return Flow{}, err
	}
	n, err := q.UpdateCapitalFlowPointer(ctx, sqlcgen.UpdateCapitalFlowPointerParams{
		CurrentRevisionID: mustBin(revisionID),
		UpdatedAt:         now,
		PortfolioID:       mustBin(current.PortfolioID),
		ID:                mustBin(current.ID),
		Version:           version,
	})
	if err != nil {
		return Flow{}, err
	}
	if n == 0 {
		latest, getErr := s.GetFlow(ctx, current.PortfolioID, current.ID)
		if getErr != nil {
			return Flow{}, getErr
		}
		return latest, ErrPrecondition
	}
	item, err := s.getFlow(ctx, q, current.PortfolioID, current.ID)
	if err != nil {
		return Flow{}, err
	}
	if err := insertAudit(ctx, q, current.PortfolioID, actor, requestID, action, "capital_flow", current.ID, current, item, now); err != nil {
		return Flow{}, err
	}
	if err := tx.Commit(); err != nil {
		return Flow{}, err
	}
	return item, nil
}

func (s *Service) getFlow(ctx context.Context, q *sqlcgen.Queries, portfolioID, flowID uuid.UUID) (Flow, error) {
	head, err := q.GetCapitalFlow(ctx, sqlcgen.GetCapitalFlowParams{PortfolioID: mustBin(portfolioID), ID: mustBin(flowID)})
	if errors.Is(err, sql.ErrNoRows) {
		return Flow{}, ErrNotFound
	}
	if err != nil {
		return Flow{}, err
	}
	if len(head.CurrentRevisionID) != 16 {
		return Flow{}, ErrNotFound
	}
	rev, err := q.GetCapitalFlowRevision(ctx, sqlcgen.GetCapitalFlowRevisionParams{PortfolioID: mustBin(portfolioID), ID: head.CurrentRevisionID})
	if err != nil {
		return Flow{}, err
	}
	return flowFromHead(&head, &rev)
}

func (in *PatchFlowInput) empty() bool {
	return in.Direction == nil && in.Quantity == nil && in.AssetID == nil && in.EffectiveAt == nil && in.AccountID == nil && !in.ClearAccount && in.Note == nil
}

func flowRevisionParams(portfolioID, flowID, revisionID uuid.UUID, draft *revisionDraft, note *string, state string, actor uuid.UUID, now time.Time) sqlcgen.InsertCapitalFlowRevisionParams {
	params := sqlcgen.InsertCapitalFlowRevisionParams{
		ID:              mustBin(revisionID),
		PortfolioID:     mustBin(portfolioID),
		CapitalFlowID:   mustBin(flowID),
		Direction:       draft.Direction,
		Quantity:        draft.Quantity,
		AssetID:         mustBin(draft.AssetID),
		EffectiveAt:     draft.EffectiveAt,
		Note:            nullNote(note),
		State:           state,
		CreatedByUserID: mustBin(actor),
		CreatedAt:       now,
	}
	if draft.AccountID != nil {
		params.AccountID = mustBin(*draft.AccountID)
	}
	return params
}

func insertAudit(ctx context.Context, q *sqlcgen.Queries, portfolioID, actor uuid.UUID, requestID, action, resourceType string, resourceID uuid.UUID, before, after any, now time.Time) error {
	auditID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	var beforeJSON, afterJSON json.RawMessage
	if before != nil {
		beforeJSON, err = json.Marshal(before)
		if err != nil {
			return err
		}
	}
	if after != nil {
		afterJSON, err = json.Marshal(after)
		if err != nil {
			return err
		}
	}
	return q.InsertAuditEvent(ctx, sqlcgen.InsertAuditEventParams{
		ID:           mustBin(auditID),
		PortfolioID:  mustBin(portfolioID),
		ActorUserID:  mustBin(actor),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   mustBin(resourceID),
		RequestID:    defaultString(requestID, "unknown"),
		BeforeJson:   sqljson.Column(beforeJSON),
		AfterJson:    sqljson.Column(afterJSON),
		OccurredAt:   now,
	})
}

func currentOrNil(exists bool, current *Baseline) any {
	if !exists {
		return nil
	}
	return current
}

func baselineFrom(head *sqlcgen.CapitalBaseline, rev *sqlcgen.CapitalBaselineRevision) (Baseline, error) {
	id, err := uuid.FromBytes(head.ID)
	if err != nil {
		return Baseline{}, err
	}
	portfolioID, err := uuid.FromBytes(head.PortfolioID)
	if err != nil {
		return Baseline{}, err
	}
	revisionID, err := uuid.FromBytes(rev.ID)
	if err != nil {
		return Baseline{}, err
	}
	assetID, err := uuid.FromBytes(rev.AssetID)
	if err != nil {
		return Baseline{}, err
	}
	return Baseline{
		ID:                id,
		PortfolioID:       portfolioID,
		CurrentRevisionID: revisionID,
		Version:           head.Version,
		EffectiveAt:       rev.EffectiveAt,
		AssetID:           assetID,
		Quantity:          compactDecimal(rev.Quantity),
		Note:              ptrNote(rev.Note),
	}, nil
}

func baselineRevisionFromRow(row *sqlcgen.CapitalBaselineRevision) (BaselineRevision, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return BaselineRevision{}, err
	}
	assetID, err := uuid.FromBytes(row.AssetID)
	if err != nil {
		return BaselineRevision{}, err
	}
	actor, err := uuid.FromBytes(row.CreatedByUserID)
	if err != nil {
		return BaselineRevision{}, err
	}
	return BaselineRevision{
		ID:              id,
		EffectiveAt:     row.EffectiveAt,
		AssetID:         assetID,
		Quantity:        compactDecimal(row.Quantity),
		Note:            ptrNote(row.Note),
		CreatedByUserID: actor,
		CreatedAt:       row.CreatedAt,
	}, nil
}

func flowFromHead(head *sqlcgen.CapitalFlow, rev *sqlcgen.CapitalFlowRevision) (Flow, error) {
	id, err := uuid.FromBytes(head.ID)
	if err != nil {
		return Flow{}, err
	}
	portfolioID, err := uuid.FromBytes(head.PortfolioID)
	if err != nil {
		return Flow{}, err
	}
	revisionID, err := uuid.FromBytes(rev.ID)
	if err != nil {
		return Flow{}, err
	}
	assetID, err := uuid.FromBytes(rev.AssetID)
	if err != nil {
		return Flow{}, err
	}
	return Flow{
		ID:                id,
		PortfolioID:       portfolioID,
		CurrentRevisionID: revisionID,
		Version:           head.Version,
		Direction:         rev.Direction,
		Quantity:          compactDecimal(rev.Quantity),
		AssetID:           assetID,
		EffectiveAt:       rev.EffectiveAt,
		AccountID:         optionalID(rev.AccountID),
		Note:              ptrNote(rev.Note),
		State:             rev.State,
	}, nil
}

func flowFromListRow(row *sqlcgen.ListActiveCapitalFlowsRow) (Flow, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Flow{}, err
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Flow{}, err
	}
	revisionID, err := uuid.FromBytes(row.RevisionID)
	if err != nil {
		return Flow{}, err
	}
	assetID, err := uuid.FromBytes(row.AssetID)
	if err != nil {
		return Flow{}, err
	}
	return Flow{
		ID:                id,
		PortfolioID:       portfolioID,
		CurrentRevisionID: revisionID,
		Version:           row.Version,
		Direction:         row.Direction,
		Quantity:          compactDecimal(row.Quantity),
		AssetID:           assetID,
		EffectiveAt:       row.EffectiveAt,
		AccountID:         optionalID(row.AccountID),
		Note:              ptrNote(row.Note),
		State:             row.State,
	}, nil
}

func flowRevisionFromRow(row *sqlcgen.CapitalFlowRevision) (FlowRevision, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return FlowRevision{}, err
	}
	assetID, err := uuid.FromBytes(row.AssetID)
	if err != nil {
		return FlowRevision{}, err
	}
	actor, err := uuid.FromBytes(row.CreatedByUserID)
	if err != nil {
		return FlowRevision{}, err
	}
	return FlowRevision{
		ID:              id,
		Direction:       row.Direction,
		Quantity:        compactDecimal(row.Quantity),
		AssetID:         assetID,
		EffectiveAt:     row.EffectiveAt,
		AccountID:       optionalID(row.AccountID),
		Note:            ptrNote(row.Note),
		State:           row.State,
		CreatedByUserID: actor,
		CreatedAt:       row.CreatedAt,
	}, nil
}

func normalizeDecimal(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !decimalPattern.MatchString(value) {
		return "", ErrInvalid
	}
	return value, nil
}

func positiveDecimal(value string) (string, error) {
	normalized, err := normalizeDecimal(value)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(normalized, "-") || compactDecimal(normalized) == "0" {
		return "", ErrInvalid
	}
	return normalized, nil
}

func compactDecimal(value string) string {
	if value == "" || !strings.Contains(value, ".") {
		return value
	}
	trimmed := strings.TrimRight(value, "0")
	trimmed = strings.TrimRight(trimmed, ".")
	if trimmed == "" || trimmed == "-" {
		return "0"
	}
	return trimmed
}

func normalizePage(offset, limit int) (int, int, error) {
	if offset < 0 {
		return 0, 0, ErrInvalidPage
	}
	limit, err := normalizeLimit(limit)
	if err != nil {
		return 0, 0, err
	}
	return offset, limit, nil
}

func normalizeLimit(limit int) (int, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit {
		return 0, ErrInvalidPage
	}
	return limit, nil
}

func nullNote(note *string) sql.NullString {
	if note == nil {
		return sql.NullString{}
	}
	value := strings.TrimSpace(*note)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func ptrNote(note sql.NullString) *string {
	if !note.Valid {
		return nil
	}
	value := note.String
	return &value
}

func optionalID(b []byte) *uuid.UUID {
	if len(b) != 16 {
		return nil
	}
	id, err := uuid.FromBytes(b)
	if err != nil {
		return nil
	}
	return &id
}

func mustBin(id uuid.UUID) []byte {
	b, err := id.MarshalBinary()
	if err != nil {
		return id[:]
	}
	return b
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
