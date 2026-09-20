package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Mapping is the current listing-to-asset decision for a Portfolio.
type Mapping struct {
	ID                  uuid.UUID `json:"id"`
	PortfolioID         uuid.UUID `json:"portfolioId"`
	ListingID           uuid.UUID `json:"listingId"`
	AssetID             uuid.UUID `json:"assetId"`
	State               string    `json:"state"`
	DisplayNameOverride *string   `json:"displayNameOverride,omitempty"`
	CurrentRevisionID   uuid.UUID `json:"currentRevisionId"`
	Version             uint64    `json:"version"`
}

// PutMappingInput appends a mapping revision.
type PutMappingInput struct {
	ActorUserID         uuid.UUID
	RequestID           string
	AssetID             uuid.UUID
	State               string
	DisplayNameOverride *string
	Reason              *string
	IfMatch             string
}

// ETag returns the mapping validator.
func (m *Mapping) ETag() string {
	return fmt.Sprintf(`"listing-mapping:%d"`, m.Version)
}

// ParseMappingETag extracts the mapping version or a create wildcard.
func ParseMappingETag(value string) (uint64, bool, bool) {
	return parseRevisionETag(value, `"listing-mapping:%d"`)
}

// PutMapping creates the listing mapping or appends a correction.
func (s *Service) PutMapping(ctx context.Context, portfolioID, listingID uuid.UUID, in *PutMappingInput) (Mapping, bool, error) {
	if in == nil {
		return Mapping{}, false, ErrInvalid
	}
	if in.State != stateProvisional && in.State != stateConfirmed {
		return Mapping{}, false, ErrInvalid
	}
	if err := validateDisplayName(in.DisplayNameOverride); err != nil {
		return Mapping{}, false, err
	}
	if err := validateReason(in.Reason); err != nil {
		return Mapping{}, false, err
	}
	if _, err := s.q.GetAssetListing(ctx, mustBin(listingID)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Mapping{}, false, ErrNotFound
		}
		return Mapping{}, false, err
	}
	if _, err := s.q.GetAssetByID(ctx, mustBin(in.AssetID)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Mapping{}, false, ErrInvalid
		}
		return Mapping{}, false, err
	}
	version, star, ok := ParseMappingETag(in.IfMatch)
	if !ok {
		return Mapping{}, false, ErrPrecondition
	}
	current, err := getMapping(ctx, s.q, portfolioID, listingID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Mapping{}, false, err
	}
	exists := err == nil
	if star && exists {
		return current, false, ErrExists
	}
	if !star && !exists {
		return Mapping{}, false, ErrNotFound
	}
	if exists && current.Version != version {
		return current, false, ErrPrecondition
	}

	revisionID, err := uuid.NewV7()
	if err != nil {
		return Mapping{}, false, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Mapping{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	created := !exists
	var mappingID uuid.UUID
	if created {
		mappingID, err = uuid.NewV7()
		if err != nil {
			return Mapping{}, false, err
		}
		if err := q.InsertAssetListingMapping(ctx, sqlcgen.InsertAssetListingMappingParams{
			ID:             mustBin(mappingID),
			PortfolioID:    mustBin(portfolioID),
			AssetListingID: mustBin(listingID),
			Version:        1,
			CreatedAt:      now,
			UpdatedAt:      now,
		}); err != nil {
			if isDuplicate(err) {
				return Mapping{}, false, ErrExists
			}
			return Mapping{}, false, err
		}
	} else {
		mappingID = current.ID
	}
	if err := q.InsertAssetListingMappingRevision(ctx, sqlcgen.InsertAssetListingMappingRevisionParams{
		ID:                  mustBin(revisionID),
		PortfolioID:         mustBin(portfolioID),
		MappingID:           mustBin(mappingID),
		AssetID:             mustBin(in.AssetID),
		State:               in.State,
		DisplayNameOverride: nullNote(in.DisplayNameOverride),
		Reason:              nullNote(in.Reason),
		CreatedByUserID:     mustBin(in.ActorUserID),
		CreatedAt:           now,
	}); err != nil {
		return Mapping{}, false, err
	}
	if created {
		if err := q.SetAssetListingMappingRevision(ctx, sqlcgen.SetAssetListingMappingRevisionParams{
			CurrentRevisionID: mustBin(revisionID),
			UpdatedAt:         now,
			PortfolioID:       mustBin(portfolioID),
			ID:                mustBin(mappingID),
		}); err != nil {
			return Mapping{}, false, err
		}
	} else {
		n, err := q.UpdateAssetListingMappingPointer(ctx, sqlcgen.UpdateAssetListingMappingPointerParams{
			CurrentRevisionID: mustBin(revisionID),
			UpdatedAt:         now,
			PortfolioID:       mustBin(portfolioID),
			ID:                mustBin(mappingID),
			Version:           version,
		})
		if err != nil {
			return Mapping{}, false, err
		}
		if n == 0 {
			latest, getErr := getMapping(ctx, s.q, portfolioID, listingID)
			if getErr != nil {
				return Mapping{}, false, getErr
			}
			return latest, false, ErrPrecondition
		}
	}
	after, err := getMapping(ctx, q, portfolioID, listingID)
	if err != nil {
		return Mapping{}, false, err
	}
	action := "catalog_mapping.revise"
	if created {
		action = "catalog_mapping.create"
	}
	if err := insertAudit(ctx, q, portfolioID, in.ActorUserID, in.RequestID, action, "catalog_mapping", mappingID, currentOrNil(exists, &current), after, now); err != nil {
		return Mapping{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Mapping{}, false, err
	}
	return after, created, nil
}

func getMapping(ctx context.Context, q *sqlcgen.Queries, portfolioID, listingID uuid.UUID) (Mapping, error) {
	head, err := q.GetAssetListingMapping(ctx, sqlcgen.GetAssetListingMappingParams{
		PortfolioID:    mustBin(portfolioID),
		AssetListingID: mustBin(listingID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Mapping{}, ErrNotFound
		}
		return Mapping{}, err
	}
	if len(head.CurrentRevisionID) != 16 {
		return Mapping{}, ErrNotFound
	}
	rev, err := q.GetAssetListingMappingRevision(ctx, sqlcgen.GetAssetListingMappingRevisionParams{
		PortfolioID: mustBin(portfolioID),
		ID:          head.CurrentRevisionID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Mapping{}, ErrNotFound
		}
		return Mapping{}, err
	}
	return mappingFrom(&head, &rev)
}

func mappingFrom(head *sqlcgen.AssetListingMapping, rev *sqlcgen.AssetListingMappingRevision) (Mapping, error) {
	id, err := uuid.FromBytes(head.ID)
	if err != nil {
		return Mapping{}, err
	}
	portfolioID, err := uuid.FromBytes(head.PortfolioID)
	if err != nil {
		return Mapping{}, err
	}
	listingID, err := uuid.FromBytes(head.AssetListingID)
	if err != nil {
		return Mapping{}, err
	}
	revisionID, err := uuid.FromBytes(rev.ID)
	if err != nil {
		return Mapping{}, err
	}
	assetID, err := uuid.FromBytes(rev.AssetID)
	if err != nil {
		return Mapping{}, err
	}
	return Mapping{
		ID:                  id,
		PortfolioID:         portfolioID,
		ListingID:           listingID,
		AssetID:             assetID,
		State:               rev.State,
		DisplayNameOverride: ptrNote(rev.DisplayNameOverride),
		CurrentRevisionID:   revisionID,
		Version:             head.Version,
	}, nil
}
