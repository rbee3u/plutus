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
	rows, err := s.q.ListCurrentAssetExclusions(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		id, err := uuid.FromBytes(row.AssetID)
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

func applyExclusion(asset *Asset, state exclusionState) {
	asset.Ignored = state.Ignored
	asset.ExclusionVersion = state.Version
	if state.Ignored {
		asset.IgnoreReason = state.Reason
		asset.IgnoredAt = &state.UpdatedAt
	}
}

// withExclusion adds a Portfolio's current rule to one shared catalog asset.
func (s *Service) withExclusion(ctx context.Context, portfolioID uuid.UUID, asset *Asset) error {
	states, err := s.currentExclusions(ctx, portfolioID)
	if err != nil {
		return err
	}
	applyExclusion(asset, states[asset.ID])
	return nil
}

// ExclusionETag is the Portfolio-specific validator, separate from mapping's global ETag.
func (asset *Asset) ExclusionETag() string {
	return etag.Format("asset-exclusion", asset.ExclusionVersion)
}

// ParseExclusionETag extracts the Portfolio-specific version.
func ParseExclusionETag(value string) (uint64, bool) {
	return etag.ParseAllowZero("asset-exclusion", value)
}

// ExclusionInput changes one Portfolio's treatment of an Asset.
type ExclusionInput struct {
	PortfolioID uuid.UUID
	ActorUserID uuid.UUID
	RequestID   string
	Ignored     bool
	Reason      string
	Version     uint64
}

// PutExclusion saves a rule for cycles that start after this operation.
func (s *Service) PutExclusion(ctx context.Context, id uuid.UUID, in *ExclusionInput) (Asset, error) {
	if in == nil {
		return Asset{}, ErrInvalid
	}
	reason := strings.TrimSpace(in.Reason)
	if in.Ignored && (reason == "" || utf8.RuneCountInString(reason) > 500) {
		return Asset{}, ErrInvalid
	}
	if !in.Ignored && reason != "" {
		return Asset{}, ErrInvalid
	}
	asset, err := s.GetAsset(ctx, id)
	if err != nil {
		return Asset{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Asset{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if _, err := q.LockExclusionPortfolio(ctx, in.PortfolioID[:]); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Asset{}, ErrNotFound
		}
		return Asset{}, err
	}
	row, err := q.GetAssetExclusion(ctx, sqlcgen.GetAssetExclusionParams{PortfolioID: in.PortfolioID[:], AssetID: id[:]})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Asset{}, err
	}
	current := exclusionState{Ignored: row.Ignored, Version: row.Version, UpdatedAt: row.UpdatedAt}
	if row.Reason.Valid {
		current.Reason = &row.Reason.String
	}
	if current.Version != in.Version {
		return Asset{}, ErrPrecondition
	}
	if current.Ignored == in.Ignored {
		applyExclusion(&asset, current)
		return asset, nil
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	reasonValue := sql.NullString{String: reason, Valid: in.Ignored}
	next := current.Version + 1
	err = q.UpsertAssetExclusion(ctx, sqlcgen.UpsertAssetExclusionParams{
		PortfolioID: in.PortfolioID[:], AssetID: id[:], Ignored: in.Ignored,
		Reason: reasonValue, Version: next, ActorUserID: in.ActorUserID[:], UpdatedAt: now,
	})
	if err != nil {
		return Asset{}, err
	}
	err = q.InsertAssetExclusionEvent(ctx, sqlcgen.InsertAssetExclusionEventParams{
		PortfolioID: in.PortfolioID[:], AssetID: id[:], Version: next, Ignored: in.Ignored,
		Reason: reasonValue, ActorUserID: in.ActorUserID[:], EffectiveAt: now,
	})
	if err != nil {
		return Asset{}, err
	}
	updated := exclusionState{Ignored: in.Ignored, Version: next, UpdatedAt: now}
	if in.Ignored {
		updated.Reason = &reason
	}
	action := "catalog.asset.restore"
	if in.Ignored {
		action = "catalog.asset.exclude"
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: in.PortfolioID, ActorUserID: in.ActorUserID, Action: action,
		ResourceType: "asset", ResourceID: id, RequestID: in.RequestID,
		Before: exclusionAudit(&asset, current), After: exclusionAudit(&asset, updated),
	}); err != nil {
		return Asset{}, err
	}
	if err := tx.Commit(); err != nil {
		return Asset{}, err
	}
	applyExclusion(&asset, updated)
	return asset, nil
}

func exclusionAudit(asset *Asset, state exclusionState) map[string]any {
	return map[string]any{
		"platform": asset.Platform, "symbol": asset.Symbol,
		"ignored": state.Ignored, "reason": state.Reason, "version": state.Version,
	}
}
