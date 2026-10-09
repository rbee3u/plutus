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
	rows, err := s.q.ListCurrentPlatformAssetExclusions(ctx, portfolioID[:])
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		id, err := uuid.FromBytes(row.PlatformAssetID)
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

func applyExclusion(platformAsset *PlatformAsset, state exclusionState) {
	platformAsset.Ignored = state.Ignored
	platformAsset.ExclusionVersion = state.Version
	if state.Ignored {
		platformAsset.IgnoreReason = state.Reason
		platformAsset.IgnoredAt = &state.UpdatedAt
	}
}

// withExclusion adds a Portfolio's current rule to one shared catalog platform asset.
func (s *Service) withExclusion(ctx context.Context, portfolioID uuid.UUID, platformAsset *PlatformAsset) error {
	states, err := s.currentExclusions(ctx, portfolioID)
	if err != nil {
		return err
	}
	applyExclusion(platformAsset, states[platformAsset.ID])
	return nil
}

// ExclusionETag is the Portfolio-specific validator, separate from mapping's global ETag.
func (l *PlatformAsset) ExclusionETag() string {
	return etag.Format("platform-asset-exclusion", l.ExclusionVersion)
}

// ParseExclusionETag extracts the Portfolio-specific version.
func ParseExclusionETag(value string) (uint64, bool) {
	return etag.ParseAllowZero("platform-asset-exclusion", value)
}

// ExclusionInput changes one Portfolio's treatment of a Platform asset.
type ExclusionInput struct {
	PortfolioID uuid.UUID
	ActorUserID uuid.UUID
	RequestID   string
	Ignored     bool
	Reason      string
	Version     uint64
}

// PutExclusion saves a rule for cycles that start after this operation.
func (s *Service) PutExclusion(ctx context.Context, id uuid.UUID, in *ExclusionInput) (PlatformAsset, error) {
	if in == nil {
		return PlatformAsset{}, ErrInvalid
	}
	reason := strings.TrimSpace(in.Reason)
	if in.Ignored && (reason == "" || utf8.RuneCountInString(reason) > 500) {
		return PlatformAsset{}, ErrInvalid
	}
	if !in.Ignored && reason != "" {
		return PlatformAsset{}, ErrInvalid
	}
	platformAsset, err := s.GetPlatformAsset(ctx, id)
	if err != nil {
		return PlatformAsset{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PlatformAsset{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if _, err := q.LockExclusionPortfolio(ctx, in.PortfolioID[:]); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlatformAsset{}, ErrNotFound
		}
		return PlatformAsset{}, err
	}
	row, err := q.GetPlatformAssetExclusion(ctx, sqlcgen.GetPlatformAssetExclusionParams{PortfolioID: in.PortfolioID[:], PlatformAssetID: id[:]})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PlatformAsset{}, err
	}
	current := exclusionState{Ignored: row.Ignored, Version: row.Version, UpdatedAt: row.UpdatedAt}
	if row.Reason.Valid {
		current.Reason = &row.Reason.String
	}
	if current.Version != in.Version {
		return PlatformAsset{}, ErrPrecondition
	}
	if current.Ignored == in.Ignored {
		applyExclusion(&platformAsset, current)
		return platformAsset, nil
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	reasonValue := sql.NullString{String: reason, Valid: in.Ignored}
	next := current.Version + 1
	err = q.UpsertPlatformAssetExclusion(ctx, sqlcgen.UpsertPlatformAssetExclusionParams{
		PortfolioID: in.PortfolioID[:], PlatformAssetID: id[:], Ignored: in.Ignored,
		Reason: reasonValue, Version: next, ActorUserID: in.ActorUserID[:], UpdatedAt: now,
	})
	if err != nil {
		return PlatformAsset{}, err
	}
	err = q.InsertPlatformAssetExclusionEvent(ctx, sqlcgen.InsertPlatformAssetExclusionEventParams{
		PortfolioID: in.PortfolioID[:], PlatformAssetID: id[:], Version: next, Ignored: in.Ignored,
		Reason: reasonValue, ActorUserID: in.ActorUserID[:], EffectiveAt: now,
	})
	if err != nil {
		return PlatformAsset{}, err
	}
	updated := exclusionState{Ignored: in.Ignored, Version: next, UpdatedAt: now}
	if in.Ignored {
		updated.Reason = &reason
	}
	action := "catalog.platform_asset.restore"
	if in.Ignored {
		action = "catalog.platform_asset.exclude"
	}
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID: in.PortfolioID, ActorUserID: in.ActorUserID, Action: action,
		ResourceType: "platform_asset", ResourceID: id, RequestID: in.RequestID,
		Before: exclusionAudit(&platformAsset, current), After: exclusionAudit(&platformAsset, updated),
	}); err != nil {
		return PlatformAsset{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlatformAsset{}, err
	}
	applyExclusion(&platformAsset, updated)
	return platformAsset, nil
}

func exclusionAudit(platformAsset *PlatformAsset, state exclusionState) map[string]any {
	return map[string]any{
		"platform": platformAsset.Platform, "symbol": platformAsset.Symbol,
		"ignored": state.Ignored, "reason": state.Reason, "version": state.Version,
	}
}
