package catalog

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	minLimit     = 1
	maxLimit     = 200
	defaultLimit = 50
)

// ErrInvalidPage is returned when offset pagination is out of range.
var ErrInvalidPage = errors.New("page is invalid")

// Service reads shared catalog rows.
type Service struct {
	q *sqlcgen.Queries
}

// NewService constructs a catalog service.
func NewService(db *sql.DB) *Service {
	return &Service{q: sqlcgen.New(db)}
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

// ErrNotMapped is returned when a listing has no confirmed mapping.
var ErrNotMapped = errors.New("asset listing is not confirmed")

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
