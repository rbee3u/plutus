// Package catalog owns Asset Listings, Instruments, and Platform prices.
package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
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

var symbolPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]{0,63}$`)

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

// Listing is one Platform coin symbol and the Asset it maps to.
type Listing struct {
	ID            uuid.UUID
	Platform      string
	Symbol        string
	AssetSymbol   string
	Name          *string
	MappingSource string
	FirstSeenAt   time.Time
	LastSeenAt    time.Time
	Version       uint64
	UpdatedAt     time.Time
}

// ETag returns the listing validator.
func (l *Listing) ETag() string {
	return fmt.Sprintf(`"listing:%d"`, l.Version)
}

// ParseListingETag extracts the version from a listing ETag.
func ParseListingETag(value string) (uint64, bool) {
	var version uint64
	if _, err := fmt.Sscanf(value, `"listing:%d"`, &version); err != nil {
		return 0, false
	}
	return version, true
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
	Platform string
	Query    string
	Offset   int
	Limit    int
}

// NormalizeSymbol upper-cases and validates a Platform symbol.
func NormalizeSymbol(raw string) (string, error) {
	symbol := strings.ToUpper(strings.TrimSpace(raw))
	if symbol == "" || len(symbol) > maxSymbol || !symbolPattern.MatchString(symbol) {
		return "", ErrInvalid
	}
	return symbol, nil
}

// AssetSymbolFor derives the automatic Asset symbol from a Platform listing symbol (ADR 0018).
func AssetSymbolFor(symbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	// Platform-specific wrappers such as "LDBTC" or "BETHUSDT" are left as-is; only the
	// common lowercase and whitespace differences are removed so that USDT on every Platform
	// merges into one Asset.
	return upper
}

// EnsureListing upserts a listing inside q and returns its ID and current Asset symbol.
func (s *Service) EnsureListing(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, symbol, name string, seenAt time.Time) (uuid.UUID, string, error) {
	normalized, err := NormalizeSymbol(symbol)
	if err != nil {
		return uuid.Nil, "", err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, "", err
	}
	var nullName sql.NullString
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		nullName = sql.NullString{String: trimmed, Valid: true}
	}
	if err := q.UpsertAssetListing(ctx, sqlcgen.UpsertAssetListingParams{
		ID:          id[:],
		Platform:    string(platform),
		Symbol:      normalized,
		AssetSymbol: AssetSymbolFor(normalized),
		Name:        nullName,
		SeenAt:      seenAt.UTC(),
	}); err != nil {
		return uuid.Nil, "", err
	}
	row, err := q.GetAssetListingByPlatformSymbol(ctx, sqlcgen.GetAssetListingByPlatformSymbolParams{Platform: string(platform), Symbol: normalized})
	if err != nil {
		return uuid.Nil, "", err
	}
	stored, err := uuid.FromBytes(row.ID)
	if err != nil {
		return uuid.Nil, "", err
	}
	return stored, row.AssetSymbol, nil
}

// EnsureInstrument upserts an instrument inside q and returns its ID.
func (s *Service) EnsureInstrument(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, in *adapters.Instrument, seenAt time.Time) (uuid.UUID, error) {
	symbol, err := NormalizeSymbol(in.Symbol)
	if err != nil {
		return uuid.Nil, err
	}
	if in.Kind == "" {
		return uuid.Nil, ErrInvalid
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	params := sqlcgen.UpsertInstrumentParams{
		ID:               id[:],
		Platform:         string(platform),
		Symbol:           symbol,
		Kind:             in.Kind,
		BaseSymbol:       optionalString(in.BaseSymbol),
		QuoteSymbol:      optionalString(in.QuoteSymbol),
		SettlementSymbol: optionalString(in.SettlementSymbol),
		SeenAt:           seenAt.UTC(),
	}
	if in.ExpiryAt != nil {
		params.ExpiryAt = sql.NullTime{Time: in.ExpiryAt.UTC(), Valid: true}
	}
	if in.ContractSize != nil && decimal.Valid(*in.ContractSize) {
		params.ContractSize = sql.NullString{String: *in.ContractSize, Valid: true}
	}
	if err := q.UpsertInstrument(ctx, params); err != nil {
		return uuid.Nil, err
	}
	row, err := q.GetInstrumentByPlatformSymbol(ctx, sqlcgen.GetInstrumentByPlatformSymbolParams{Platform: string(platform), Symbol: symbol})
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.FromBytes(row.ID)
}

// RecordPrice upserts one Platform price inside q. Prices are stored in their quote symbol;
// valuation treats USDT, USDC, and USD quotes as USD (ADR 0017).
func (s *Service) RecordPrice(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, price adapters.Price) error {
	asset, err := NormalizeSymbol(price.AssetSymbol)
	if err != nil {
		return err
	}
	quote, err := NormalizeSymbol(price.QuoteSymbol)
	if err != nil {
		return err
	}
	if !decimal.Valid(price.Price) {
		return ErrInvalid
	}
	return q.UpsertPrice(ctx, sqlcgen.UpsertPriceParams{
		Platform:    string(platform),
		AssetSymbol: asset,
		ObservedAt:  price.ObservedAt.UTC(),
		Price:       price.Price,
		QuoteSymbol: quote,
	})
}

// ListListings returns an offset page of listings.
func (s *Service) ListListings(ctx context.Context, in ListFilter) (Page[Listing], error) {
	limit, offset, err := normalizePage(in.Limit, in.Offset)
	if err != nil {
		return Page[Listing]{}, err
	}
	query := likePattern(in.Query)
	rows, err := s.q.ListAssetListings(ctx, sqlcgen.ListAssetListingsParams{
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
		Limit:          int32(limit),
		Offset:         int32(offset),
	})
	if err != nil {
		return Page[Listing]{}, err
	}
	total, err := s.q.CountAssetListings(ctx, sqlcgen.CountAssetListingsParams{
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
	})
	if err != nil {
		return Page[Listing]{}, err
	}
	items := make([]Listing, 0, len(rows))
	for i := range rows {
		item, err := listingFromRow(&rows[i])
		if err != nil {
			return Page[Listing]{}, err
		}
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

// MappingInput overrides the Asset symbol of a listing.
type MappingInput struct {
	ActorUserID uuid.UUID
	PortfolioID uuid.UUID
	RequestID   string
	AssetSymbol string
	Version     uint64
}

// PutMapping sets the listing's Asset symbol. Equal to the listing symbol restores automatic mapping.
// It returns the updated listing and the Portfolios whose points hold this listing.
func (s *Service) PutMapping(ctx context.Context, id uuid.UUID, in MappingInput) (Listing, []uuid.UUID, error) {
	asset, err := NormalizeSymbol(in.AssetSymbol)
	if err != nil {
		return Listing{}, nil, err
	}
	current, err := s.GetListing(ctx, id)
	if err != nil {
		return Listing{}, nil, err
	}
	if current.Version != in.Version {
		return current, nil, ErrPrecondition
	}
	source := MappingManual
	if asset == AssetSymbolFor(current.Symbol) {
		source = MappingAuto
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Listing{}, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.UpdateAssetListingMapping(ctx, sqlcgen.UpdateAssetListingMappingParams{
		AssetSymbol:   asset,
		MappingSource: source,
		UpdatedAt:     now,
		ID:            id[:],
		Version:       in.Version,
	})
	if err != nil {
		if mysql.IsDuplicate(err) {
			return Listing{}, nil, ErrInvalid
		}
		return Listing{}, nil, err
	}
	if n == 0 {
		return current, nil, ErrPrecondition
	}
	updated := current
	updated.AssetSymbol = asset
	updated.MappingSource = source
	updated.Version++
	updated.UpdatedAt = now
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID:  in.PortfolioID,
		ActorUserID:  in.ActorUserID,
		Action:       "catalog.listing.map",
		ResourceType: "asset_listing",
		ResourceID:   id,
		RequestID:    in.RequestID,
		Before:       auditView(&current),
		After:        auditView(&updated),
	}); err != nil {
		return Listing{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return Listing{}, nil, err
	}
	holders, err := s.q.ListPortfoliosHoldingListing(ctx, id[:])
	if err != nil {
		return Listing{}, nil, err
	}
	affected := make([]uuid.UUID, 0, len(holders))
	for _, raw := range holders {
		pid, err := uuid.FromBytes(raw)
		if err != nil {
			return Listing{}, nil, err
		}
		affected = append(affected, pid)
	}
	return updated, affected, nil
}

// ListInstruments returns an offset page of instruments.
func (s *Service) ListInstruments(ctx context.Context, in ListFilter) (Page[Instrument], error) {
	limit, offset, err := normalizePage(in.Limit, in.Offset)
	if err != nil {
		return Page[Instrument]{}, err
	}
	query := likePattern(in.Query)
	rows, err := s.q.ListInstruments(ctx, sqlcgen.ListInstrumentsParams{
		FilterPlatform: flag(in.Platform != ""),
		Platform:       in.Platform,
		FilterQuery:    flag(query != ""),
		Query:          query,
		Limit:          int32(limit),
		Offset:         int32(offset),
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

type listingAudit struct {
	Platform      string `json:"platform"`
	Symbol        string `json:"symbol"`
	AssetSymbol   string `json:"assetSymbol"`
	MappingSource string `json:"mappingSource"`
	Version       uint64 `json:"version"`
}

func auditView(l *Listing) listingAudit {
	return listingAudit{Platform: l.Platform, Symbol: l.Symbol, AssetSymbol: l.AssetSymbol, MappingSource: l.MappingSource, Version: l.Version}
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
	if limit < minLimit || limit > maxLimit || offset < 0 {
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

func optionalString(value string) sql.NullString {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	s := decimal.Compact(value.String)
	return &s
}
