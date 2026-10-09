// Package catalog owns Platform Assets, Futures Contracts, and Platform prices.
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

	// MappingAuto means the Asset symbol equals the normalized platform asset symbol.
	MappingAuto = "auto"
	// MappingManual means a User overrode the Asset symbol.
	MappingManual = "manual"
)

// symbolPattern accepts letters of any script, so symbols such as Binance's 牛来USDT contract are
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

// PlatformAsset is an asset available on a Platform, with its own Symbol and a mapping to an Asset.
type PlatformAsset struct {
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

// ETag returns the platform asset validator.
func (l *PlatformAsset) ETag() string {
	return etag.Format("platform-asset", l.Version)
}

// ParsePlatformAssetETag extracts the version from a platform asset ETag.
func ParsePlatformAssetETag(value string) (uint64, bool) {
	return etag.Parse("platform-asset", value)
}

// FuturesContract is one Platform contract.
type FuturesContract struct {
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

// ListPlatformAssets returns an offset page of platform assets.
func (s *Service) ListPlatformAssets(ctx context.Context, in ListFilter) (Page[PlatformAsset], error) {
	limit, offset, err := normalizePage(in.Limit, in.Offset)
	if err != nil {
		return Page[PlatformAsset]{}, err
	}
	query := likePattern(in.Query)
	rows, err := s.q.ListPlatformAssets(ctx, sqlcgen.ListPlatformAssetsParams{
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
		return Page[PlatformAsset]{}, err
	}
	total, err := s.q.CountPlatformAssets(ctx, sqlcgen.CountPlatformAssetsParams{
		PortfolioID:    in.PortfolioID[:],
		IgnoredOnly:    flag(in.IgnoredOnly),
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
	})
	if err != nil {
		return Page[PlatformAsset]{}, err
	}
	items := make([]PlatformAsset, 0, len(rows))
	states, err := s.currentExclusions(ctx, in.PortfolioID)
	if err != nil {
		return Page[PlatformAsset]{}, err
	}
	for i := range rows {
		item, err := platformAssetFromRow(&rows[i])
		if err != nil {
			return Page[PlatformAsset]{}, err
		}
		applyExclusion(&item, states[item.ID])
		items = append(items, item)
	}
	return Page[PlatformAsset]{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

// GetPlatformAsset returns one platform asset.
func (s *Service) GetPlatformAsset(ctx context.Context, id uuid.UUID) (PlatformAsset, error) {
	row, err := s.q.GetPlatformAsset(ctx, id[:])
	if errors.Is(err, sql.ErrNoRows) {
		return PlatformAsset{}, ErrNotFound
	}
	if err != nil {
		return PlatformAsset{}, err
	}
	return platformAssetFromRow(&row)
}

// ListFuturesContracts returns an offset page of futures contracts.
func (s *Service) ListFuturesContracts(ctx context.Context, in ListFilter) (Page[FuturesContract], error) {
	limit, offset, err := normalizePage(in.Limit, in.Offset)
	if err != nil {
		return Page[FuturesContract]{}, err
	}
	query := likePattern(in.Query)
	rows, err := s.q.ListFuturesContracts(ctx, sqlcgen.ListFuturesContractsParams{
		PlatformOrder:  ordering.PlatformsCSV(),
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
		Limit:          int32(limit),  // #nosec G115 -- normalizePage validates the SQL int32 pagination bounds.
		Offset:         int32(offset), // #nosec G115 -- normalizePage validates the SQL int32 pagination bounds.
	})
	if err != nil {
		return Page[FuturesContract]{}, err
	}
	total, err := s.q.CountFuturesContracts(ctx, sqlcgen.CountFuturesContractsParams{
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
	})
	if err != nil {
		return Page[FuturesContract]{}, err
	}
	items := make([]FuturesContract, 0, len(rows))
	for i := range rows {
		item, err := futuresContractFromRow(&rows[i])
		if err != nil {
			return Page[FuturesContract]{}, err
		}
		items = append(items, item)
	}
	return Page[FuturesContract]{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

func platformAssetFromRow(row *sqlcgen.PlatformAsset) (PlatformAsset, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return PlatformAsset{}, err
	}
	item := PlatformAsset{
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

func futuresContractFromRow(row *sqlcgen.FuturesContract) (FuturesContract, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return FuturesContract{}, err
	}
	item := FuturesContract{
		ID:               id,
		Platform:         row.Platform,
		Symbol:           row.Symbol,
		Kind:             row.Kind,
		BaseSymbol:       nullStringPtr(row.BasePlatformAssetSymbol),
		QuoteSymbol:      nullStringPtr(row.QuotePlatformAssetSymbol),
		SettlementSymbol: nullStringPtr(row.SettlementPlatformAssetSymbol),
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
