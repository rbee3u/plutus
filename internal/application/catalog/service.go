// Package catalog owns Assets, Futures Contracts, and Platform prices.
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

	// MappingAuto means the Canonical Asset Symbol equals the normalized Asset Symbol.
	MappingAuto = "auto"
	// MappingManual means a User overrode the Canonical Asset Symbol.
	MappingManual = "manual"
)

// symbolPattern accepts letters of any script, so symbols such as Binance's 牛来USDT contract are
// kept; maxSymbol bounds the byte length, which keeps it within the VARCHAR(64) columns.
var symbolPattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}._-]*$`)

// Service reads and writes the Platform catalog.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a catalog service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// Asset is an asset available on a Platform, with its own Symbol and a mapping to a Canonical Asset.
type Asset struct {
	ID                   uuid.UUID
	Platform             string
	Symbol               string
	CanonicalAssetSymbol string
	MappingSource        string
	FirstObservedAt      time.Time
	LastObservedAt       time.Time
	MappingVersion       uint64
	Excluded             bool
	ExclusionReason      *string
	ExcludedAt           *time.Time
	ExclusionVersion     uint64
}

// ETag returns the asset validator.
func (asset *Asset) ETag() string {
	return etag.Format("asset", asset.MappingVersion)
}

// ParseAssetETag extracts the version from an asset ETag.
func ParseAssetETag(value string) (uint64, bool) {
	return etag.Parse("asset", value)
}

// FuturesContract is one Platform symbol with one immutable set of observed terms.
type FuturesContract struct {
	ID                    uuid.UUID
	Platform              string
	Symbol                string
	Kind                  string
	BaseAssetSymbol       *string
	QuoteAssetSymbol      *string
	SettlementAssetSymbol *string
	ExpiryAt              *time.Time
	ContractSize          *string
	FirstObservedAt       time.Time
	LastObservedAt        time.Time
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
	Status      string
	Offset      int
	Limit       int
}

// ListAssets returns an offset page of assets.
func (s *Service) ListAssets(ctx context.Context, in ListFilter) (Page[Asset], error) {
	limit, offset, err := normalizePage(in.Limit, in.Offset)
	if err != nil {
		return Page[Asset]{}, err
	}
	query := likePattern(in.Query)
	filterStatus := flag(in.Status == "active" || in.Status == "excluded")
	rows, err := s.q.ListAssets(ctx, sqlcgen.ListAssetsParams{
		PlatformOrder:  ordering.PlatformsCSV(),
		PortfolioID:    in.PortfolioID[:],
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
		FilterStatus:   filterStatus,
		Status:         in.Status,
		Limit:          int32(limit),  // #nosec G115 -- normalizePage validates the SQL int32 pagination bounds.
		Offset:         int32(offset), // #nosec G115 -- normalizePage validates the SQL int32 pagination bounds.
	})
	if err != nil {
		return Page[Asset]{}, err
	}
	total, err := s.q.CountAssets(ctx, sqlcgen.CountAssetsParams{
		PortfolioID:    in.PortfolioID[:],
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
		FilterStatus:   filterStatus,
		Status:         in.Status,
	})
	if err != nil {
		return Page[Asset]{}, err
	}
	items := make([]Asset, 0, len(rows))
	states, err := s.currentExclusions(ctx, in.PortfolioID)
	if err != nil {
		return Page[Asset]{}, err
	}
	for i := range rows {
		item, err := assetFromRow(&rows[i])
		if err != nil {
			return Page[Asset]{}, err
		}
		applyExclusion(&item, states[item.ID])
		items = append(items, item)
	}
	return Page[Asset]{Items: items, Offset: offset, Limit: limit, Total: int(total)}, nil
}

// GetAsset returns one asset.
func (s *Service) GetAsset(ctx context.Context, portfolioID, id uuid.UUID) (Asset, error) {
	row, err := s.q.GetAsset(ctx, sqlcgen.GetAssetParams{PortfolioID: portfolioID[:], ID: id[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	if err != nil {
		return Asset{}, err
	}
	return assetFromRow(&row)
}

// ListFuturesContracts returns an offset page of futures contracts.
func (s *Service) ListFuturesContracts(ctx context.Context, in ListFilter) (Page[FuturesContract], error) {
	limit, offset, err := normalizePage(in.Limit, in.Offset)
	if err != nil {
		return Page[FuturesContract]{}, err
	}
	query := likePattern(in.Query)
	rows, err := s.q.ListFuturesContracts(ctx, sqlcgen.ListFuturesContractsParams{
		PortfolioID:    in.PortfolioID[:],
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
		PortfolioID:    in.PortfolioID[:],
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

func assetFromRow(row *sqlcgen.Asset) (Asset, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Asset{}, err
	}
	item := Asset{
		ID:                   id,
		Platform:             row.Platform,
		Symbol:               row.Symbol,
		CanonicalAssetSymbol: row.CanonicalAssetSymbol,
		MappingSource:        MappingAuto,
		FirstObservedAt:      row.FirstObservedAt,
		LastObservedAt:       row.LastObservedAt,
		MappingVersion:       row.MappingVersion,
	}
	if row.CanonicalAssetSymbol != CanonicalAssetSymbolFor(row.Symbol) {
		item.MappingSource = MappingManual
	}
	return item, nil
}

func futuresContractFromRow(row *sqlcgen.ListFuturesContractsRow) (FuturesContract, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return FuturesContract{}, err
	}
	item := FuturesContract{
		ID:                    id,
		Platform:              row.Platform,
		Symbol:                row.Symbol,
		Kind:                  row.Kind,
		BaseAssetSymbol:       nullStringPtr(row.BaseAssetSymbol),
		QuoteAssetSymbol:      nullStringPtr(row.QuoteAssetSymbol),
		SettlementAssetSymbol: nullStringPtr(row.SettlementAssetSymbol),
		ContractSize:          nullDecimalPtr(row.ContractSize),
		FirstObservedAt:       row.FirstObservedAt,
		LastObservedAt:        row.LastObservedAt,
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
	return new(value.String)
}

func nullDecimalPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return new(decimal.Compact(value.String))
}
