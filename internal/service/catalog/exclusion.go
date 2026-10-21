package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/service/audit"
)

type exclusionState struct {
	Excluded  bool
	Reason    *string
	Version   uint64
	UpdatedAt time.Time
}

func applyExclusion(asset *Asset, state exclusionState) {
	asset.Excluded = state.Excluded
	asset.ExclusionVersion = state.Version
	if state.Excluded {
		asset.ExclusionReason = state.Reason
		asset.ExcludedAt = &state.UpdatedAt
	}
}

// withExclusion adds a portfolio's current rule to one local catalog asset.
func (s *Service) withExclusion(ctx context.Context, portfolioID uuid.UUID, asset *Asset) error {
	row, err := s.q.GetAssetExclusion(ctx, sqlcgen.GetAssetExclusionParams{PortfolioID: portfolioID[:], AssetID: asset.ID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	state := exclusionState{Excluded: row.Excluded, Version: row.Version, UpdatedAt: row.EffectiveAt}
	if row.Reason.Valid {
		state.Reason = &row.Reason.String
	}
	applyExclusion(asset, state)
	return nil
}

// ExclusionInput changes one portfolio's treatment of an asset.
type ExclusionInput struct {
	PortfolioID uuid.UUID
	ActorUserID uuid.UUID
	RequestID   string
	Excluded    bool
	Reason      string
	Version     uint64
}

// PutExclusion saves a rule for cycles that start after this operation.
func (s *Service) PutExclusion(ctx context.Context, id uuid.UUID, in *ExclusionInput) (Asset, error) {
	if in == nil {
		return Asset{}, ErrInvalid
	}
	reason := strings.TrimSpace(in.Reason)
	if in.Excluded && (reason == "" || utf8.RuneCountInString(reason) > 500) {
		return Asset{}, ErrInvalid
	}
	if !in.Excluded && reason != "" {
		return Asset{}, ErrInvalid
	}
	asset, err := s.GetAsset(ctx, in.PortfolioID, id)
	if err != nil {
		return Asset{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Asset{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if _, err := q.LockPortfolio(ctx, in.PortfolioID[:]); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Asset{}, ErrNotFound
		}
		return Asset{}, err
	}
	row, err := q.GetAssetExclusion(ctx, sqlcgen.GetAssetExclusionParams{PortfolioID: in.PortfolioID[:], AssetID: id[:]})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Asset{}, err
	}
	current := exclusionState{Excluded: row.Excluded, Version: row.Version, UpdatedAt: row.EffectiveAt}
	if row.Reason.Valid {
		current.Reason = &row.Reason.String
	}
	if current.Version != in.Version {
		return Asset{}, ErrPrecondition
	}
	if current.Excluded == in.Excluded {
		applyExclusion(&asset, current)
		return asset, nil
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	reasonValue := sql.NullString{String: reason, Valid: in.Excluded}
	next := current.Version + 1
	err = q.InsertAssetExclusionVersion(ctx, sqlcgen.InsertAssetExclusionVersionParams{
		PortfolioID: in.PortfolioID[:], AssetID: id[:], Version: next, Excluded: in.Excluded,
		Reason: reasonValue, EffectiveAt: now,
	})
	if err != nil {
		return Asset{}, err
	}
	updated := exclusionState{Excluded: in.Excluded, Version: next, UpdatedAt: now}
	if in.Excluded {
		updated.Reason = &reason
	}
	action := "catalog.asset.restore"
	if in.Excluded {
		action = "catalog.asset.exclude"
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: in.PortfolioID, ActorUserID: in.ActorUserID, Action: action,
		ResourceType: "asset", ResourceID: id, RequestID: in.RequestID,
		Before: map[string]any{"version": current.Version}, After: map[string]any{"version": next},
	}); err != nil {
		return Asset{}, err
	}
	if err := tx.Commit(); err != nil {
		return Asset{}, err
	}
	applyExclusion(&asset, updated)
	return asset, nil
}
