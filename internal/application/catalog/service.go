// Package catalog owns Asset Listings, Instruments, and Platform prices.
package catalog

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/jobs"
	"github.com/rbee3u/plutus/internal/application/ordering"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
	"github.com/rbee3u/plutus/internal/shared/etag"
)

const (
	minLimit     = 1
	maxLimit     = 200
	defaultLimit = 50
	maxSymbol    = 64

	// MappingAuto means the Asset symbol equals the normalized listing symbol.
	MappingAuto = "auto"
	// MappingManual means a User overrode the Asset symbol.
	MappingManual = "manual"
)

// symbolPattern accepts letters of any script, so listings such as Binance's 牛来USDT contract are
// kept; maxSymbol bounds the byte length, which keeps it within the VARCHAR(64) columns.
var symbolPattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}._-]*$`)

// Service reads and writes the Platform catalog.
type Service struct {
	jobs *jobs.Service
	db   *sql.DB
	q    *sqlcgen.Queries
	now  func() time.Time
}

// NewService constructs a catalog service.
func NewService(db *sql.DB, jobService *jobs.Service) *Service {
	return &Service{jobs: jobService, db: db, q: sqlcgen.New(db), now: time.Now}
}

// Listing is one Platform coin symbol and the Asset it maps to.
type Listing struct {
	ID               uuid.UUID
	Platform         string
	Symbol           string
	AssetSymbol      string
	Name             *string
	MappingSource    string
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
	Version          uint64
	UpdatedAt        time.Time
	Ignored          bool
	IgnoreReason     *string
	IgnoredAt        *time.Time
	ExclusionVersion uint64
}

// ETag returns the listing validator.
func (l *Listing) ETag() string {
	return etag.Format("listing", l.Version)
}

// ParseListingETag extracts the version from a listing ETag.
func ParseListingETag(value string) (uint64, bool) {
	return etag.Parse("listing", value)
}

// Instrument is one Platform contract.
type Instrument struct {
	ID               uuid.UUID
	Platform         string
	Symbol           string
	Kind             string
	BaseSymbol       *string
	QuoteSymbol      *string
	SettlementSymbol *string
	ExpiryAt         *time.Time
	ContractSize     *string
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
}

// Page is an offset page.
type Page[T any] struct {
	Items  []T
	Offset int
	Limit  int
	Total  int
}

// ListFilter selects catalog rows.
type ListFilter struct {
	PortfolioID uuid.UUID
	Platform    string
	Query       string
	IgnoredOnly bool
	Offset      int
	Limit       int
}

// ListListings returns an offset page of listings.
func (s *Service) ListListings(ctx context.Context, in ListFilter) (Page[Listing], error) {
	limit, offset, err := normalizePage(in.Limit, in.Offset)
	if err != nil {
		return Page[Listing]{}, err
	}
	query := likePattern(in.Query)
	rows, err := s.q.ListAssetListings(ctx, sqlcgen.ListAssetListingsParams{
		PlatformOrder:  ordering.PlatformsCSV(),
		PortfolioID:    in.PortfolioID[:],
		IgnoredOnly:    flag(in.IgnoredOnly),
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
		Limit:          int32(limit),  // #nosec G115 -- normalizePage validates the SQL int32 pagination bounds.
		Offset:         int32(offset), // #nosec G115 -- normalizePage validates the SQL int32 pagination bounds.
	})
	if err != nil {
		return Page[Listing]{}, err
	}
	total, err := s.q.CountAssetListings(ctx, sqlcgen.CountAssetListingsParams{
		PortfolioID:    in.PortfolioID[:],
		IgnoredOnly:    flag(in.IgnoredOnly),
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
	})
	if err != nil {
		return Page[Listing]{}, err
	}
	items := make([]Listing, 0, len(rows))
	states, err := s.currentExclusions(ctx, in.PortfolioID)
	if err != nil {
		return Page[Listing]{}, err
	}
	for i := range rows {
		item, err := listingFromRow(&rows[i])
		if err != nil {
			return Page[Listing]{}, err
		}
		applyExclusion(&item, states[item.ID])
		items = append(items, item)
	}
	return Page[Listing]{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

// GetListing returns one listing.
func (s *Service) GetListing(ctx context.Context, id uuid.UUID) (Listing, error) {
	row, err := s.q.GetAssetListing(ctx, id[:])
	if errors.Is(err, sql.ErrNoRows) {
		return Listing{}, ErrNotFound
	}
	if err != nil {
		return Listing{}, err
	}
	return listingFromRow(&row)
}

// ListInstruments returns an offset page of instruments.
func (s *Service) ListInstruments(ctx context.Context, in ListFilter) (Page[Instrument], error) {
	limit, offset, err := normalizePage(in.Limit, in.Offset)
	if err != nil {
		return Page[Instrument]{}, err
	}
	query := likePattern(in.Query)
	rows, err := s.q.ListInstruments(ctx, sqlcgen.ListInstrumentsParams{
		PlatformOrder:  ordering.PlatformsCSV(),
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
		Limit:          int32(limit),  // #nosec G115 -- normalizePage validates the SQL int32 pagination bounds.
		Offset:         int32(offset), // #nosec G115 -- normalizePage validates the SQL int32 pagination bounds.
	})
	if err != nil {
		return Page[Instrument]{}, err
	}
	total, err := s.q.CountInstruments(ctx, sqlcgen.CountInstrumentsParams{
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
	})
	if err != nil {
		return Page[Instrument]{}, err
	}
	items := make([]Instrument, 0, len(rows))
	for i := range rows {
		item, err := instrumentFromRow(&rows[i])
		if err != nil {
			return Page[Instrument]{}, err
		}
		items = append(items, item)
	}
	return Page[Instrument]{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

func listingFromRow(row *sqlcgen.AssetListing) (Listing, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Listing{}, err
	}
	item := Listing{
		ID:            id,
		Platform:      row.Platform,
		Symbol:        row.Symbol,
		AssetSymbol:   row.AssetSymbol,
		MappingSource: row.MappingSource,
		FirstSeenAt:   row.FirstSeenAt,
		LastSeenAt:    row.LastSeenAt,
		Version:       row.Version,
		UpdatedAt:     row.UpdatedAt,
	}
	if row.Name.Valid {
		name := row.Name.String
		item.Name = &name
	}
	return item, nil
}

func instrumentFromRow(row *sqlcgen.Instrument) (Instrument, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Instrument{}, err
	}
	item := Instrument{
		ID:               id,
		Platform:         row.Platform,
		Symbol:           row.Symbol,
		Kind:             row.Kind,
		BaseSymbol:       nullStringPtr(row.BaseSymbol),
		QuoteSymbol:      nullStringPtr(row.QuoteSymbol),
		SettlementSymbol: nullStringPtr(row.SettlementSymbol),
		ContractSize:     nullStringPtr(row.ContractSize),
		FirstSeenAt:      row.FirstSeenAt,
		LastSeenAt:       row.LastSeenAt,
	}
	if row.ExpiryAt.Valid {
		t := row.ExpiryAt.Time
		item.ExpiryAt = &t
	}
	return item, nil
}

func normalizePage(limit, offset int) (int, int, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit || offset < 0 || offset > math.MaxInt32 {
		return 0, 0, ErrInvalid
	}
	return limit, offset, nil
}

func likePattern(query string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(query))
	if trimmed == "" {
		return ""
	}
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(trimmed)
	return "%" + escaped + "%"
}

func flag(on bool) int {
	if on {
		return 1
	}
	return 0
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return new(decimal.Compact(value.String))
}
