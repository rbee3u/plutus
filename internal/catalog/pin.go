package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Pin is the current tracking decision for a listing or instrument.
type Pin struct {
	ID                uuid.UUID `json:"id"`
	PortfolioID       uuid.UUID `json:"portfolioId"`
	SubjectKind       string    `json:"subjectKind"`
	SubjectID         uuid.UUID `json:"subjectId"`
	Enabled           bool      `json:"enabled"`
	CurrentRevisionID uuid.UUID `json:"currentRevisionId"`
	Version           uint64    `json:"version"`
}

// PutPinInput appends an enabled tracking revision.
type PutPinInput struct {
	ActorUserID uuid.UUID
	RequestID   string
	Reason      *string
	IfMatch     string
}

// DisablePinInput appends a disabled tracking revision.
type DisablePinInput struct {
	ActorUserID uuid.UUID
	RequestID   string
	Reason      *string
	IfMatch     string
}

// ETag returns the pin validator.
func (p *Pin) ETag() string {
	return fmt.Sprintf(`"catalog-pin:%d"`, p.Version)
}

// ParsePinETag extracts the pin version or a create wildcard.
func ParsePinETag(value string) (uint64, bool, bool) {
	return parseRevisionETag(value, `"catalog-pin:%d"`)
}

// PutPin creates the pin or appends an enabled revision.
func (s *Service) PutPin(ctx context.Context, portfolioID uuid.UUID, subjectKind string, subjectID uuid.UUID, in *PutPinInput) (Pin, bool, error) {
	if in == nil {
		return Pin{}, false, ErrInvalid
	}
	if err := validateReason(in.Reason); err != nil {
		return Pin{}, false, err
	}
	kind, err := normalizeSubjectKind(subjectKind)
	if err != nil {
		return Pin{}, false, err
	}
	if err := s.ensureSubject(ctx, kind, subjectID); err != nil {
		return Pin{}, false, err
	}
	return s.appendPin(ctx, portfolioID, kind, subjectID, &pinRevisionInput{
		enabled:     true,
		actor:       in.ActorUserID,
		requestID:   in.RequestID,
		reason:      in.Reason,
		ifMatch:     in.IfMatch,
		allowCreate: true,
	})
}

// DisablePin appends a disabled tracking revision.
func (s *Service) DisablePin(ctx context.Context, portfolioID uuid.UUID, subjectKind string, subjectID uuid.UUID, in *DisablePinInput) (Pin, error) {
	if in == nil {
		return Pin{}, ErrInvalid
	}
	if err := validateReason(in.Reason); err != nil {
		return Pin{}, err
	}
	kind, err := normalizeSubjectKind(subjectKind)
	if err != nil {
		return Pin{}, err
	}
	item, _, err := s.appendPin(ctx, portfolioID, kind, subjectID, &pinRevisionInput{
		enabled:     false,
		actor:       in.ActorUserID,
		requestID:   in.RequestID,
		reason:      in.Reason,
		ifMatch:     in.IfMatch,
		allowCreate: false,
	})
	return item, err
}

type pinRevisionInput struct {
	enabled     bool
	actor       uuid.UUID
	requestID   string
	reason      *string
	ifMatch     string
	allowCreate bool
}

func (s *Service) appendPin(ctx context.Context, portfolioID uuid.UUID, kind string, subjectID uuid.UUID, in *pinRevisionInput) (Pin, bool, error) {
	if in == nil {
		return Pin{}, false, ErrInvalid
	}
	version, star, ok := ParsePinETag(in.ifMatch)
	if !ok {
		return Pin{}, false, ErrPrecondition
	}
	if star && !in.allowCreate {
		return Pin{}, false, ErrPrecondition
	}
	current, err := getPin(ctx, s.q, portfolioID, kind, subjectID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Pin{}, false, err
	}
	exists := err == nil
	if star && exists {
		return current, false, ErrExists
	}
	if !star && !exists {
		return Pin{}, false, ErrNotFound
	}
	if exists && current.Version != version {
		return current, false, ErrPrecondition
	}

	revisionID, err := uuid.NewV7()
	if err != nil {
		return Pin{}, false, err
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Pin{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	created := !exists
	var pinID uuid.UUID
	if created {
		pinID, err = uuid.NewV7()
		if err != nil {
			return Pin{}, false, err
		}
		params := sqlcgen.InsertCatalogPinParams{
			ID:          mustBin(pinID),
			PortfolioID: mustBin(portfolioID),
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if kind == subjectListing {
			params.AssetListingID = mustBin(subjectID)
		} else {
			params.InstrumentID = mustBin(subjectID)
		}
		if err := q.InsertCatalogPin(ctx, params); err != nil {
			if isDuplicate(err) {
				return Pin{}, false, ErrExists
			}
			return Pin{}, false, err
		}
	} else {
		pinID = current.ID
	}
	if err := q.InsertCatalogPinRevision(ctx, sqlcgen.InsertCatalogPinRevisionParams{
		ID:              mustBin(revisionID),
		PortfolioID:     mustBin(portfolioID),
		CatalogPinID:    mustBin(pinID),
		Enabled:         in.enabled,
		Reason:          nullNote(in.reason),
		CreatedByUserID: mustBin(in.actor),
		CreatedAt:       now,
	}); err != nil {
		return Pin{}, false, err
	}
	if created {
		if err := q.SetCatalogPinRevision(ctx, sqlcgen.SetCatalogPinRevisionParams{
			CurrentRevisionID: mustBin(revisionID),
			UpdatedAt:         now,
			PortfolioID:       mustBin(portfolioID),
			ID:                mustBin(pinID),
		}); err != nil {
			return Pin{}, false, err
		}
	} else {
		n, err := q.UpdateCatalogPinPointer(ctx, sqlcgen.UpdateCatalogPinPointerParams{
			CurrentRevisionID: mustBin(revisionID),
			UpdatedAt:         now,
			PortfolioID:       mustBin(portfolioID),
			ID:                mustBin(pinID),
			Version:           version,
		})
		if err != nil {
			return Pin{}, false, err
		}
		if n == 0 {
			latest, getErr := getPin(ctx, s.q, portfolioID, kind, subjectID)
			if getErr != nil {
				return Pin{}, false, getErr
			}
			return latest, false, ErrPrecondition
		}
	}
	after, err := getPin(ctx, q, portfolioID, kind, subjectID)
	if err != nil {
		return Pin{}, false, err
	}
	action := "catalog_pin.revise"
	switch {
	case created:
		action = "catalog_pin.create"
	case !in.enabled:
		action = "catalog_pin.disable"
	}
	if err := insertAudit(ctx, q, portfolioID, in.actor, in.requestID, action, "catalog_pin", pinID, currentOrNil(exists, &current), after, now); err != nil {
		return Pin{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Pin{}, false, err
	}
	return after, created, nil
}

func (s *Service) ensureSubject(ctx context.Context, kind string, subjectID uuid.UUID) error {
	if kind == subjectListing {
		_, err := s.q.GetAssetListing(ctx, mustBin(subjectID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_, err := s.q.GetInstrument(ctx, mustBin(subjectID))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func getPin(ctx context.Context, q *sqlcgen.Queries, portfolioID uuid.UUID, kind string, subjectID uuid.UUID) (Pin, error) {
	head, err := getPinHead(ctx, q, portfolioID, kind, subjectID)
	if err != nil {
		return Pin{}, err
	}
	if len(head.CurrentRevisionID) != 16 {
		return Pin{}, ErrNotFound
	}
	rev, err := q.GetCatalogPinRevision(ctx, sqlcgen.GetCatalogPinRevisionParams{
		PortfolioID: mustBin(portfolioID),
		ID:          head.CurrentRevisionID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Pin{}, ErrNotFound
		}
		return Pin{}, err
	}
	return pinFrom(&head, &rev)
}

type pinHead struct {
	ID                []byte
	PortfolioID       []byte
	AssetListingID    []byte
	InstrumentID      []byte
	CurrentRevisionID []byte
	Version           uint64
}

func getPinHead(ctx context.Context, q *sqlcgen.Queries, portfolioID uuid.UUID, kind string, subjectID uuid.UUID) (pinHead, error) {
	if kind == subjectListing {
		row, err := q.GetCatalogPinByListing(ctx, sqlcgen.GetCatalogPinByListingParams{
			PortfolioID:    mustBin(portfolioID),
			AssetListingID: mustBin(subjectID),
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return pinHead{}, ErrNotFound
			}
			return pinHead{}, err
		}
		return pinHead{
			ID:                row.ID,
			PortfolioID:       row.PortfolioID,
			AssetListingID:    row.AssetListingID,
			InstrumentID:      row.InstrumentID,
			CurrentRevisionID: row.CurrentRevisionID,
			Version:           row.Version,
		}, nil
	}
	row, err := q.GetCatalogPinByInstrument(ctx, sqlcgen.GetCatalogPinByInstrumentParams{
		PortfolioID:  mustBin(portfolioID),
		InstrumentID: mustBin(subjectID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pinHead{}, ErrNotFound
		}
		return pinHead{}, err
	}
	return pinHead{
		ID:                row.ID,
		PortfolioID:       row.PortfolioID,
		AssetListingID:    row.AssetListingID,
		InstrumentID:      row.InstrumentID,
		CurrentRevisionID: row.CurrentRevisionID,
		Version:           row.Version,
	}, nil
}

func pinFrom(head *pinHead, rev *sqlcgen.CatalogPinRevision) (Pin, error) {
	id, err := uuid.FromBytes(head.ID)
	if err != nil {
		return Pin{}, err
	}
	portfolioID, err := uuid.FromBytes(head.PortfolioID)
	if err != nil {
		return Pin{}, err
	}
	revisionID, err := uuid.FromBytes(rev.ID)
	if err != nil {
		return Pin{}, err
	}
	kind, subjectID, err := pinSubject(head.AssetListingID, head.InstrumentID)
	if err != nil {
		return Pin{}, err
	}
	return Pin{
		ID:                id,
		PortfolioID:       portfolioID,
		SubjectKind:       kind,
		SubjectID:         subjectID,
		Enabled:           rev.Enabled,
		CurrentRevisionID: revisionID,
		Version:           head.Version,
	}, nil
}

func pinSubject(listingID, instrumentID []byte) (string, uuid.UUID, error) {
	if listing := optionalUUID(listingID); listing != nil {
		return subjectListing, *listing, nil
	}
	if instrument := optionalUUID(instrumentID); instrument != nil {
		return subjectInstrument, *instrument, nil
	}
	return "", uuid.Nil, ErrInvalid
}

func normalizeSubjectKind(kind string) (string, error) {
	switch kind {
	case subjectListing, subjectInstrument:
		return kind, nil
	default:
		return "", ErrInvalidSubject
	}
}
