package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/audit"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/etag"
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
	rows, err := s.q.ListCurrentListingExclusions(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		id, err := uuid.FromBytes(row.ListingID)
		if err != nil {
			return nil, err
		}
		state := exclusionState{Ignored: row.Ignored, Version: row.Version, UpdatedAt: row.UpdatedAt}
		if row.Reason.Valid {
			state.Reason = &row.Reason.String
		}
		states[id] = state
	}
	return states, nil
}

func applyExclusion(listing *Listing, state exclusionState) {
	listing.Ignored = state.Ignored
	listing.ExclusionVersion = state.Version
	if state.Ignored {
		listing.IgnoreReason = state.Reason
		listing.IgnoredAt = &state.UpdatedAt
	}
}

// withExclusion adds a Portfolio's current rule to one shared catalog listing.
func (s *Service) withExclusion(ctx context.Context, portfolioID uuid.UUID, listing *Listing) error {
	states, err := s.currentExclusions(ctx, portfolioID)
	if err != nil {
		return err
	}
	applyExclusion(listing, states[listing.ID])
	return nil
}

// ExclusionETag is the Portfolio-specific validator, separate from mapping's global ETag.
func (l *Listing) ExclusionETag() string {
	return etag.Format("listing-exclusion", l.ExclusionVersion)
}

// ParseExclusionETag extracts the Portfolio-specific version.
func ParseExclusionETag(value string) (uint64, bool) {
	return etag.ParseAllowZero("listing-exclusion", value)
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
	q := s.q.WithTx(tx)
	if _, err := q.LockExclusionPortfolio(ctx, in.PortfolioID[:]); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, err
	}
	row, err := q.GetListingExclusion(ctx, sqlcgen.GetListingExclusionParams{PortfolioID: in.PortfolioID[:], ListingID: id[:]})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Listing{}, err
	}
	current := exclusionState{Ignored: row.Ignored, Version: row.Version, UpdatedAt: row.UpdatedAt}
	if row.Reason.Valid {
		current.Reason = &row.Reason.String
	}
	if current.Version != in.Version {
		return Listing{}, ErrPrecondition
	}
	if current.Ignored == in.Ignored {
		applyExclusion(&listing, current)
		return listing, nil
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	reasonValue := sql.NullString{String: reason, Valid: in.Ignored}
	next := current.Version + 1
	err = q.UpsertListingExclusion(ctx, sqlcgen.UpsertListingExclusionParams{
		PortfolioID: in.PortfolioID[:], ListingID: id[:], Ignored: in.Ignored,
		Reason: reasonValue, Version: next, ActorUserID: in.ActorUserID[:], UpdatedAt: now,
	})
	if err != nil {
		return Listing{}, err
	}
	err = q.InsertListingExclusionEvent(ctx, sqlcgen.InsertListingExclusionEventParams{
		PortfolioID: in.PortfolioID[:], ListingID: id[:], Version: next, Ignored: in.Ignored,
		Reason: reasonValue, ActorUserID: in.ActorUserID[:], EffectiveAt: now,
	})
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
	if err := audit.Record(ctx, q, now, &audit.Entry{
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
