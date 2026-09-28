package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/audit"
)

type exclusionState struct {
	Ignored   bool
	Reason    *string
	Version   uint64
	UpdatedAt time.Time
}

func (s *Service) currentExclusions(ctx context.Context, portfolioID uuid.UUID) (map[uuid.UUID]exclusionState, error) {
	states := map[uuid.UUID]exclusionState{}
	if portfolioID == uuid.Nil {
		return states, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT listing_id, ignored, reason, version, updated_at
		FROM portfolio_listing_exclusions WHERE portfolio_id = ?`, portfolioID[:])
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var raw []byte
		var reason sql.NullString
		var state exclusionState
		if err := rows.Scan(&raw, &state.Ignored, &reason, &state.Version, &state.UpdatedAt); err != nil {
			return nil, err
		}
		id, err := uuid.FromBytes(raw)
		if err != nil {
			return nil, err
		}
		if reason.Valid {
			state.Reason = &reason.String
		}
		states[id] = state
	}
	return states, rows.Err()
}

func applyExclusion(listing *Listing, state exclusionState) {
	listing.Ignored = state.Ignored
	listing.ExclusionVersion = state.Version
	if state.Ignored {
		listing.IgnoreReason = state.Reason
		listing.IgnoredAt = &state.UpdatedAt
	}
}

// WithExclusion adds a Portfolio's current rule to one shared catalog listing.
func (s *Service) WithExclusion(ctx context.Context, portfolioID uuid.UUID, listing *Listing) error {
	states, err := s.currentExclusions(ctx, portfolioID)
	if err != nil {
		return err
	}
	applyExclusion(listing, states[listing.ID])
	return nil
}

// ExclusionETag is the Portfolio-specific validator, separate from mapping's global ETag.
func (l *Listing) ExclusionETag() string {
	return fmt.Sprintf(`"listing-exclusion:%d"`, l.ExclusionVersion)
}

// ParseExclusionETag extracts the Portfolio-specific version.
func ParseExclusionETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"listing-exclusion:%d"`, &version); err != nil {
		return 0, false
	}
	return version, fmt.Sprintf(`"listing-exclusion:%d"`, version) == value
}

// ExclusionInput changes one Portfolio's treatment of a Platform listing.
type ExclusionInput struct {
	PortfolioID uuid.UUID
	ActorUserID uuid.UUID
	RequestID   string
	Ignored     bool
	Reason      string
	Version     uint64
}

// PutExclusion saves a rule for cycles that start after this operation.
func (s *Service) PutExclusion(ctx context.Context, id uuid.UUID, in *ExclusionInput) (Listing, error) {
	if in == nil {
		return Listing{}, ErrInvalid
	}
	reason := strings.TrimSpace(in.Reason)
	if in.Ignored && (reason == "" || utf8.RuneCountInString(reason) > 500) {
		return Listing{}, ErrInvalid
	}
	if !in.Ignored && reason != "" {
		return Listing{}, ErrInvalid
	}
	listing, err := s.GetListing(ctx, id)
	if err != nil {
		return Listing{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Listing{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var portfolio []byte
	if err := tx.QueryRowContext(ctx, "SELECT id FROM portfolios WHERE id = ? FOR UPDATE", in.PortfolioID[:]).Scan(&portfolio); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, err
	}
	var current exclusionState
	var storedReason sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT ignored, reason, version, updated_at FROM portfolio_listing_exclusions
		WHERE portfolio_id = ? AND listing_id = ?`, in.PortfolioID[:], id[:]).Scan(&current.Ignored, &storedReason, &current.Version, &current.UpdatedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Listing{}, err
	}
	if storedReason.Valid {
		current.Reason = &storedReason.String
	}
	if current.Version != in.Version {
		return Listing{}, ErrPrecondition
	}
	if current.Ignored == in.Ignored {
		applyExclusion(&listing, current)
		return listing, nil
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	var reasonValue any
	if in.Ignored {
		reasonValue = reason
	}
	next := current.Version + 1
	_, err = tx.ExecContext(ctx, `INSERT INTO portfolio_listing_exclusions
		(portfolio_id, listing_id, ignored, reason, version, actor_user_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE ignored = VALUES(ignored), reason = VALUES(reason),
		version = VALUES(version), actor_user_id = VALUES(actor_user_id), updated_at = VALUES(updated_at)`,
		in.PortfolioID[:], id[:], in.Ignored, reasonValue, next, in.ActorUserID[:], now)
	if err != nil {
		return Listing{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO portfolio_listing_exclusion_events
		(portfolio_id, listing_id, version, ignored, reason, actor_user_id, effective_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, in.PortfolioID[:], id[:], next, in.Ignored, reasonValue, in.ActorUserID[:], now)
	if err != nil {
		return Listing{}, err
	}
	updated := exclusionState{Ignored: in.Ignored, Version: next, UpdatedAt: now}
	if in.Ignored {
		updated.Reason = &reason
	}
	action := "catalog.listing.restore"
	if in.Ignored {
		action = "catalog.listing.exclude"
	}
	if err := audit.Record(ctx, s.q.WithTx(tx), now, &audit.Entry{
		PortfolioID: in.PortfolioID, ActorUserID: in.ActorUserID, Action: action,
		ResourceType: "asset_listing", ResourceID: id, RequestID: in.RequestID,
		Before: exclusionAudit(&listing, current), After: exclusionAudit(&listing, updated),
	}); err != nil {
		return Listing{}, err
	}
	if err := tx.Commit(); err != nil {
		return Listing{}, err
	}
	applyExclusion(&listing, updated)
	return listing, nil
}

func exclusionAudit(listing *Listing, state exclusionState) map[string]any {
	return map[string]any{
		"platform": listing.Platform, "symbol": listing.Symbol,
		"ignored": state.Ignored, "reason": state.Reason, "version": state.Version,
	}
}
