package catalog

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	minLimit     = 1
	maxLimit     = 200
	defaultLimit = 50
)

// Service reads shared catalog rows and records portfolio mapping and pin decisions.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a catalog service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// Asset is a canonical catalog asset.
type Asset struct {
	ID       uuid.UUID
	Code     string
	Name     string
	Kind     string
	Decimals *int
	Status   string
}

// AssetPage is an offset page of assets.
type AssetPage struct {
	Items  []Asset
	Offset int
	Limit  int
	Total  int
}

// Listing is a discovered platform asset listing plus mapping status.
type Listing struct {
	ID              uuid.UUID
	PlatformID      uuid.UUID
	PlatformCode    string
	ExternalRef     string
	Symbol          string
	Name            *string
	Status          string
	MappedAssetID   *uuid.UUID
	MappedAssetCode *string
	MappingState    string
	MappingVersion  *uint64
	Pinned          bool
}

// ListingPage is an offset page of listings.
type ListingPage struct {
	Items  []Listing
	Offset int
	Limit  int
	Total  int
}

// ListingSearch filters discovered listings.
type ListingSearch struct {
	Query        string
	PlatformID   *uuid.UUID
	MappingState string
	Offset       int
	Limit        int
}

// Instrument is a discovered platform instrument plus pin status.
type Instrument struct {
	ID           uuid.UUID
	PlatformID   uuid.UUID
	PlatformCode string
	ExternalRef  string
	Symbol       string
	Kind         string
	Status       string
	ExpiryAt     *time.Time
	Pinned       bool
}

// InstrumentPage is an offset page of instruments.
type InstrumentPage struct {
	Items  []Instrument
	Offset int
	Limit  int
	Total  int
}

// InstrumentSearch filters discovered instruments.
type InstrumentSearch struct {
	Query      string
	PlatformID *uuid.UUID
	Pinned     *bool
	Offset     int
	Limit      int
}

// SearchAssets returns active and provisional assets matching q.
func (s *Service) SearchAssets(ctx context.Context, q string, offset, limit int) (AssetPage, error) {
	offset, limit, err := normalizePage(offset, limit)
	if err != nil {
		return AssetPage{}, err
	}
	total, err := s.q.CountSearchAssets(ctx, sqlcgen.CountSearchAssetsParams{Query: q})
	if err != nil {
		return AssetPage{}, err
	}
	rows, err := s.q.SearchAssets(ctx, sqlcgen.SearchAssetsParams{
		Query:  q,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return AssetPage{}, err
	}
	items := make([]Asset, 0, len(rows))
	for i := range rows {
		item, err := assetFromRow(&rows[i])
		if err != nil {
			return AssetPage{}, err
		}
		items = append(items, item)
	}
	return AssetPage{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

// SearchListings returns discovered listings with this Portfolio's mapping and pin status.
func (s *Service) SearchListings(ctx context.Context, portfolioID uuid.UUID, in *ListingSearch) (ListingPage, error) {
	if in == nil {
		in = &ListingSearch{}
	}
	offset, limit, err := normalizePage(in.Offset, in.Limit)
	if err != nil {
		return ListingPage{}, err
	}
	state := in.MappingState
	if state != "" && state != stateUnmapped && state != stateProvisional && state != stateConfirmed {
		return ListingPage{}, ErrInvalid
	}
	params := sqlcgen.CountSearchAssetListingsParams{
		PortfolioID:    mustBin(portfolioID),
		Query:          in.Query,
		FilterPlatform: flagInt(in.PlatformID != nil),
		PlatformID:     make([]byte, 16),
		MappingState:   state,
	}
	if in.PlatformID != nil {
		params.PlatformID = mustBin(*in.PlatformID)
	}
	total, err := s.q.CountSearchAssetListings(ctx, params)
	if err != nil {
		return ListingPage{}, err
	}
	rows, err := s.q.SearchAssetListings(ctx, sqlcgen.SearchAssetListingsParams{
		PortfolioID:    params.PortfolioID,
		Query:          params.Query,
		FilterPlatform: params.FilterPlatform,
		PlatformID:     params.PlatformID,
		MappingState:   params.MappingState,
		Limit:          int32(limit),
		Offset:         int32(offset),
	})
	if err != nil {
		return ListingPage{}, err
	}
	items := make([]Listing, 0, len(rows))
	for i := range rows {
		item, err := listingFromRow(&rows[i])
		if err != nil {
			return ListingPage{}, err
		}
		items = append(items, item)
	}
	return ListingPage{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

// SearchInstruments returns discovered instruments with this Portfolio's pin status.
func (s *Service) SearchInstruments(ctx context.Context, portfolioID uuid.UUID, in *InstrumentSearch) (InstrumentPage, error) {
	if in == nil {
		in = &InstrumentSearch{}
	}
	offset, limit, err := normalizePage(in.Offset, in.Limit)
	if err != nil {
		return InstrumentPage{}, err
	}
	params := sqlcgen.CountSearchInstrumentsParams{
		PortfolioID:    mustBin(portfolioID),
		Query:          in.Query,
		FilterPlatform: flagInt(in.PlatformID != nil),
		PlatformID:     make([]byte, 16),
		FilterPinned:   flagInt(in.Pinned != nil),
		Pinned:         0,
	}
	if in.PlatformID != nil {
		params.PlatformID = mustBin(*in.PlatformID)
	}
	if in.Pinned != nil {
		params.Pinned = flagInt(*in.Pinned)
	}
	total, err := s.q.CountSearchInstruments(ctx, params)
	if err != nil {
		return InstrumentPage{}, err
	}
	rows, err := s.q.SearchInstruments(ctx, sqlcgen.SearchInstrumentsParams{
		PortfolioID:    params.PortfolioID,
		Query:          params.Query,
		FilterPlatform: params.FilterPlatform,
		PlatformID:     params.PlatformID,
		FilterPinned:   params.FilterPinned,
		Pinned:         params.Pinned,
		Limit:          int32(limit),
		Offset:         int32(offset),
	})
	if err != nil {
		return InstrumentPage{}, err
	}
	items := make([]Instrument, 0, len(rows))
	for i := range rows {
		item, err := instrumentFromRow(&rows[i])
		if err != nil {
			return InstrumentPage{}, err
		}
		items = append(items, item)
	}
	return InstrumentPage{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

// ConfirmedMappedAsset returns the confirmed canonical asset for a listing, if any.
func (s *Service) ConfirmedMappedAsset(ctx context.Context, portfolioID, listingID uuid.UUID) (uuid.UUID, error) {
	raw, err := s.q.GetConfirmedMappingAsset(ctx, sqlcgen.GetConfirmedMappingAssetParams{
		PortfolioID:    mustBin(portfolioID),
		AssetListingID: mustBin(listingID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, ErrNotMapped
	}
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.FromBytes(raw)
}

func assetFromRow(row *sqlcgen.Asset) (Asset, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Asset{}, err
	}
	item := Asset{ID: id, Code: row.Code, Name: row.Name, Kind: row.Kind, Status: row.Status}
	if row.Decimals.Valid {
		d := int(row.Decimals.Int16)
		item.Decimals = &d
	}
	return item, nil
}

func listingFromRow(row *sqlcgen.SearchAssetListingsRow) (Listing, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Listing{}, err
	}
	platformID, err := uuid.FromBytes(row.PlatformID)
	if err != nil {
		return Listing{}, err
	}
	item := Listing{
		ID:           id,
		PlatformID:   platformID,
		PlatformCode: row.PlatformCode,
		ExternalRef:  row.ExternalRef,
		Symbol:       row.Symbol,
		Name:         ptrNote(row.Name),
		Status:       row.Status,
		MappingState: asString(row.MappingState),
		Pinned:       row.Pinned == 1,
	}
	if item.MappingState == "" {
		item.MappingState = stateUnmapped
	}
	if mapped := optionalUUID(row.MappedAssetID); mapped != nil {
		item.MappedAssetID = mapped
	}
	if row.MappedAssetCode.Valid {
		code := row.MappedAssetCode.String
		item.MappedAssetCode = &code
	}
	if row.MappingVersion > 0 {
		version := row.MappingVersion
		item.MappingVersion = &version
	}
	return item, nil
}

func instrumentFromRow(row *sqlcgen.SearchInstrumentsRow) (Instrument, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Instrument{}, err
	}
	platformID, err := uuid.FromBytes(row.PlatformID)
	if err != nil {
		return Instrument{}, err
	}
	item := Instrument{
		ID:           id,
		PlatformID:   platformID,
		PlatformCode: row.PlatformCode,
		ExternalRef:  row.ExternalRef,
		Symbol:       row.Symbol,
		Kind:         row.Kind,
		Status:       row.Status,
		Pinned:       row.Pinned == 1,
	}
	if row.ExpiryAt.Valid {
		expiry := row.ExpiryAt.Time
		item.ExpiryAt = &expiry
	}
	return item, nil
}

func normalizePage(offset, limit int) (int, int, error) {
	if offset < 0 {
		return 0, 0, ErrInvalidPage
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit {
		return 0, 0, ErrInvalidPage
	}
	return offset, limit, nil
}

func mustBin(id uuid.UUID) []byte {
	b, err := id.MarshalBinary()
	if err != nil {
		return id[:]
	}
	return b
}
