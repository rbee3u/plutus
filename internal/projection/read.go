package projection

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

const (
	maxLimit     = 200
	defaultLimit = 50
)

var (
	balanceGroups  = map[string]bool{"platform": true, "identity": true, "account": true, "asset": true, "listing": true}
	positionGroups = map[string]bool{"platform": true, "identity": true, "account": true, "instrument": true}
)

// Overview is the Phase 2 read model for the main dashboard.
type Overview struct {
	PortfolioID        uuid.UUID
	Context            Context
	Valuation          Amount
	ContributedCapital Amount
	ValueChange        Amount
	ValuationSeries    []ValuationPoint
	ValueChanges       []ValueChangePoint
	Exposures          []ExposurePoint
	Issues             []Issue
}

// Context describes the active generation used by a read model.
type Context struct {
	Timezone       string
	DisplayAssetID uuid.UUID
	DisplayCode    string
	CutoffAt       *time.Time
	Completeness   string
	Coverage       string
	Freshness      string
	GenerationID   *uuid.UUID
	RebuildPending bool
}

// Amount is a nullable financial value with status dimensions.
type Amount struct {
	Amount          *string
	Completeness    string
	Coverage        string
	Freshness       string
	EvaluatedAt     *time.Time
	State           string
	PeriodKind      string
	PeriodStartDate *time.Time
	PeriodEndDate   *time.Time
}

// ValuationPoint is one valuation sample.
type ValuationPoint struct {
	EvaluatedAt  time.Time
	Amount       *string
	Completeness string
}

// ValueChangePoint is one calendar value-change sample.
type ValueChangePoint struct {
	PeriodKind             string
	PeriodStart, PeriodEnd time.Time
	Amount                 *string
	State                  string
}

// ExposurePoint is one asset exposure at the overview cutoff.
type ExposurePoint struct {
	AssetID                         uuid.UUID
	AssetCode                       string
	Long, Short, Net                string
	LongValue, ShortValue, NetValue *string
	Completeness, Freshness         string
}

// Issue is a stable projection diagnostic.
type Issue struct {
	ID, IdentityID, AccountID, ObservationID *uuid.UUID
	Kind, Key, Code, Severity, Summary       string
	First, Last                              time.Time
}

// BalanceRow is one latest-balance projection.
type BalanceRow struct {
	IdentityID, AccountID, ListingID, PlatformID uuid.UUID
	AssetID                                      *uuid.UUID
	IdentityName, AccountName, PlatformCode      string
	ListingSymbol, AssetCode                     string
	Total, Available, Locked                     *string
	ObservedAt                                   time.Time
	Freshness, Completeness                      string
}

// PositionRow is one latest-position projection.
type PositionRow struct {
	IdentityID, AccountID, InstrumentID, PlatformID uuid.UUID
	IdentityName, AccountName, PlatformCode, Symbol string
	Ref, Side, Quantity, Long, Short                string
	Mark                                            *string
	ObservedAt                                      time.Time
	Freshness, Completeness                         string
}

// Group is a server-computed grouping total.
type Group struct {
	Grouping, Key, Label, Completeness, Freshness string
	Totals                                        GroupTotals
}

// GroupTotals holds nullable summed quantities.
type GroupTotals struct {
	Total, Available, Locked, Quantity, Long, Short *string
	LongQuantity, ShortQuantity, NetQuantity        *string
	LongValue, ShortValue, NetValue                 *string
}

// BalancePage is a filtered, grouped latest-balance response.
type BalancePage struct {
	Items   []BalanceRow
	Groups  []Group
	Offset  int
	Limit   int
	Total   int
	Context Context
	Issues  []Issue
}

// PositionPage is a filtered, grouped latest-position response.
type PositionPage struct {
	Items   []PositionRow
	Groups  []Group
	Offset  int
	Limit   int
	Total   int
	Context Context
	Issues  []Issue
}

// ListFilter is the accepted balances/positions query.
type ListFilter struct {
	Offset, Limit                                           int
	GroupBy                                                 string
	IdentityID, AccountID, AssetID, ListingID, InstrumentID *uuid.UUID
}

type readMeta struct {
	identities  map[uuid.UUID]sqlcgen.Identity
	accounts    map[uuid.UUID]sqlcgen.Account
	listings    map[uuid.UUID]sqlcgen.AssetListing
	instruments map[uuid.UUID]sqlcgen.Instrument
	platforms   map[uuid.UUID]sqlcgen.Platform
	assets      map[uuid.UUID]sqlcgen.Asset
	mappings    map[uuid.UUID]uuid.UUID
}

// Overview returns cards, series, exposure, and issues for the active generation.
func (s *Service) Overview(ctx context.Context, portfolioID uuid.UUID) (Overview, error) {
	snap, err := s.loadSnapshot(ctx, portfolioID)
	if err != nil {
		return Overview{}, err
	}
	out := Overview{PortfolioID: portfolioID, Context: snap.context, Issues: snap.issues}
	if snap.generationID == uuid.Nil {
		out.Valuation = missingAmount()
		out.ContributedCapital = missingAmount()
		out.ValueChange = missingAmount()
		return out, nil
	}
	vals, err := s.q.ListPortfolioValuationProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return Overview{}, err
	}
	capitals, err := s.q.ListContributedCapitalProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return Overview{}, err
	}
	changes, err := s.q.ListValueChangeProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return Overview{}, err
	}
	exposures, err := s.q.ListExposureProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return Overview{}, err
	}
	latestVal := latestValuation(vals)
	if latestVal != nil {
		out.Valuation = amountFromValuation(latestVal)
		out.Context.Completeness = latestVal.Completeness
		out.Context.Freshness = latestVal.FreshnessState
		out.Context.Coverage = coverageFromCompleteness(latestVal.Completeness)
		cutoff := latestVal.EvaluatedAt
		out.Context.CutoffAt = &cutoff
	} else {
		out.Valuation = missingAmount()
	}
	out.ContributedCapital = capitalAt(capitals, out.Valuation.EvaluatedAt)
	out.ValueChange = currentMonthChange(changes, snap.cutoff)
	for i := range vals {
		out.ValuationSeries = append(out.ValuationSeries, ValuationPoint{
			EvaluatedAt: vals[i].EvaluatedAt, Amount: nullCompact(vals[i].Amount), Completeness: vals[i].Completeness,
		})
	}
	for i := range changes {
		if changes[i].PeriodKind != "month" {
			continue
		}
		out.ValueChanges = append(out.ValueChanges, ValueChangePoint{
			PeriodKind: changes[i].PeriodKind, PeriodStart: changes[i].PeriodStartDate, PeriodEnd: changes[i].PeriodEndDate,
			Amount: nullCompact(changes[i].Amount), State: changes[i].State,
		})
	}
	target := time.Time{}
	if out.Valuation.EvaluatedAt != nil {
		target = *out.Valuation.EvaluatedAt
	}
	for i := range exposures {
		if !target.IsZero() && !exposures[i].EvaluatedAt.Equal(target) {
			continue
		}
		assetID, err := uuid.FromBytes(exposures[i].AssetID)
		if err != nil {
			return Overview{}, err
		}
		out.Exposures = append(out.Exposures, ExposurePoint{
			AssetID: assetID, AssetCode: snap.assetCode(assetID),
			Long: compactDecimal(exposures[i].LongQuantity), Short: compactDecimal(exposures[i].ShortQuantity), Net: compactDecimal(exposures[i].NetQuantity),
			LongValue: nullCompact(exposures[i].LongValue), ShortValue: nullCompact(exposures[i].ShortValue), NetValue: nullCompact(exposures[i].NetValue),
			Completeness: exposures[i].Completeness, Freshness: exposures[i].FreshnessState,
		})
	}
	return out, nil
}

// ListBalances returns filtered latest balances with whitelist grouping.
func (s *Service) ListBalances(ctx context.Context, portfolioID uuid.UUID, in *ListFilter) (BalancePage, error) {
	if in == nil {
		in = &ListFilter{}
	}
	if in.GroupBy != "" && !balanceGroups[in.GroupBy] {
		return BalancePage{}, ErrInvalidGroup
	}
	offset, limit, err := normalizePage(in.Offset, in.Limit)
	if err != nil {
		return BalancePage{}, err
	}
	snap, err := s.loadSnapshot(ctx, portfolioID)
	if err != nil {
		return BalancePage{}, err
	}
	page := BalancePage{Offset: offset, Limit: limit, Context: snap.context, Issues: snap.issues, Groups: []Group{}}
	if snap.generationID == uuid.Nil {
		return page, nil
	}
	rows, err := s.q.ListLatestBalanceProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return BalancePage{}, err
	}
	items := make([]BalanceRow, 0, len(rows))
	for i := range rows {
		item, err := snap.balanceRow(&rows[i])
		if err != nil {
			return BalancePage{}, err
		}
		if !matchBalance(&item, in) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].AccountID != items[j].AccountID {
			return items[i].AccountID.String() < items[j].AccountID.String()
		}
		return items[i].ListingID.String() < items[j].ListingID.String()
	})
	page.Total = len(items)
	page.Groups = groupBalances(items, in.GroupBy)
	page.Items = pageSlice(items, offset, limit)
	page.Context = mergeRowContext(&page.Context, items)
	return page, nil
}

// ListPositions returns filtered latest positions with whitelist grouping.
func (s *Service) ListPositions(ctx context.Context, portfolioID uuid.UUID, in *ListFilter) (PositionPage, error) {
	if in == nil {
		in = &ListFilter{}
	}
	if in.GroupBy != "" && !positionGroups[in.GroupBy] {
		return PositionPage{}, ErrInvalidGroup
	}
	offset, limit, err := normalizePage(in.Offset, in.Limit)
	if err != nil {
		return PositionPage{}, err
	}
	snap, err := s.loadSnapshot(ctx, portfolioID)
	if err != nil {
		return PositionPage{}, err
	}
	page := PositionPage{Offset: offset, Limit: limit, Context: snap.context, Issues: snap.issues, Groups: []Group{}}
	if snap.generationID == uuid.Nil {
		return page, nil
	}
	rows, err := s.q.ListLatestPositionProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return PositionPage{}, err
	}
	items := make([]PositionRow, 0, len(rows))
	for i := range rows {
		item, err := snap.positionRow(&rows[i])
		if err != nil {
			return PositionPage{}, err
		}
		if !matchPosition(&item, in) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].AccountID != items[j].AccountID {
			return items[i].AccountID.String() < items[j].AccountID.String()
		}
		if items[i].InstrumentID != items[j].InstrumentID {
			return items[i].InstrumentID.String() < items[j].InstrumentID.String()
		}
		return items[i].Ref < items[j].Ref
	})
	page.Total = len(items)
	page.Groups = groupPositions(items, in.GroupBy)
	page.Items = pageSlice(items, offset, limit)
	page.Context = mergePositionContext(&page.Context, items)
	return page, nil
}

type snapshot struct {
	context      Context
	generationID uuid.UUID
	cutoff       time.Time
	issues       []Issue
	meta         readMeta
}

func (s *Service) loadSnapshot(ctx context.Context, portfolioID uuid.UUID) (snapshot, error) {
	port, err := s.q.GetPortfolioByID(ctx, mustBin(portfolioID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return snapshot{}, err
		}
		return snapshot{}, err
	}
	displayID, err := uuid.FromBytes(port.DisplayAssetID)
	if err != nil {
		return snapshot{}, err
	}
	out := snapshot{
		context: Context{
			Timezone: port.Timezone, DisplayAssetID: displayID, DisplayCode: port.DisplayAssetCode,
			Completeness: "missing", Coverage: "missing", Freshness: "unknown",
		},
	}
	if len(port.ActiveProjectionGenerationID) == 16 {
		genID, err := uuid.FromBytes(port.ActiveProjectionGenerationID)
		if err != nil {
			return snapshot{}, err
		}
		out.generationID = genID
		out.context.GenerationID = &genID
		gen, err := s.q.GetProjectionGeneration(ctx, sqlcgen.GetProjectionGenerationParams{
			PortfolioID: mustBin(portfolioID), ID: mustBin(genID),
		})
		if err != nil {
			return snapshot{}, err
		}
		out.context.Timezone = gen.Timezone
		if display, err := uuid.FromBytes(gen.DisplayAssetID); err == nil {
			out.context.DisplayAssetID = display
		}
		out.cutoff = gen.SourceWatermarkAt
		cutoff := gen.SourceWatermarkAt
		out.context.CutoffAt = &cutoff
		out.context.RebuildPending = gen.Timezone != port.Timezone
		issues, err := s.q.ListProjectionIssues(ctx, mustBin(genID))
		if err != nil {
			return snapshot{}, err
		}
		out.issues = issuesFromRows(issues)
		if out.context.RebuildPending {
			out.issues = append([]Issue{{
				Kind: "portfolio_valuation", Key: "timezone", Code: "rebuild_pending", Severity: "info",
				Summary: "A timezone or display-asset change is waiting for a new projection generation.",
				First:   s.now().UTC(), Last: s.now().UTC(),
			}}, out.issues...)
		}
	} else {
		out.issues = []Issue{{
			Kind: "portfolio_valuation", Key: "generation", Code: "incomplete_inputs", Severity: "warning",
			Summary: "No active projection generation is available.", First: s.now().UTC(), Last: s.now().UTC(),
		}}
	}
	meta, err := s.loadReadMeta(ctx, portfolioID)
	if err != nil {
		return snapshot{}, err
	}
	out.meta = meta
	if asset, ok := meta.assets[out.context.DisplayAssetID]; ok {
		out.context.DisplayCode = asset.Code
	}
	return out, nil
}

func (s *Service) loadReadMeta(ctx context.Context, portfolioID uuid.UUID) (readMeta, error) {
	out := readMeta{
		identities: map[uuid.UUID]sqlcgen.Identity{}, accounts: map[uuid.UUID]sqlcgen.Account{},
		listings: map[uuid.UUID]sqlcgen.AssetListing{}, instruments: map[uuid.UUID]sqlcgen.Instrument{},
		platforms: map[uuid.UUID]sqlcgen.Platform{}, assets: map[uuid.UUID]sqlcgen.Asset{},
		mappings: map[uuid.UUID]uuid.UUID{},
	}
	identities, err := s.q.ListIdentitiesByPortfolio(ctx, mustBin(portfolioID))
	if err != nil {
		return readMeta{}, err
	}
	for i := range identities {
		id, err := uuid.FromBytes(identities[i].ID)
		if err != nil {
			return readMeta{}, err
		}
		out.identities[id] = identities[i]
	}
	accounts, err := s.q.ListAllAccountsByPortfolio(ctx, mustBin(portfolioID))
	if err != nil {
		return readMeta{}, err
	}
	for i := range accounts {
		id, err := uuid.FromBytes(accounts[i].ID)
		if err != nil {
			return readMeta{}, err
		}
		out.accounts[id] = accounts[i]
	}
	listings, err := s.q.ListAssetListings(ctx)
	if err != nil {
		return readMeta{}, err
	}
	for i := range listings {
		id, err := uuid.FromBytes(listings[i].ID)
		if err != nil {
			return readMeta{}, err
		}
		out.listings[id] = listings[i]
	}
	instruments, err := s.q.ListInstruments(ctx)
	if err != nil {
		return readMeta{}, err
	}
	for i := range instruments {
		id, err := uuid.FromBytes(instruments[i].ID)
		if err != nil {
			return readMeta{}, err
		}
		out.instruments[id] = instruments[i]
	}
	platforms, err := s.q.ListPlatforms(ctx)
	if err != nil {
		return readMeta{}, err
	}
	for i := range platforms {
		id, err := uuid.FromBytes(platforms[i].ID)
		if err != nil {
			return readMeta{}, err
		}
		out.platforms[id] = platforms[i]
	}
	assets, err := s.q.ListActiveAssets(ctx)
	if err != nil {
		return readMeta{}, err
	}
	for i := range assets {
		id, err := uuid.FromBytes(assets[i].ID)
		if err != nil {
			return readMeta{}, err
		}
		out.assets[id] = assets[i]
	}
	mappings, err := s.q.ListCurrentAssetMappings(ctx, mustBin(portfolioID))
	if err != nil {
		return readMeta{}, err
	}
	for i := range mappings {
		listingID, err := uuid.FromBytes(mappings[i].AssetListingID)
		if err != nil {
			return readMeta{}, err
		}
		assetID, err := uuid.FromBytes(mappings[i].AssetID)
		if err != nil {
			return readMeta{}, err
		}
		out.mappings[listingID] = assetID
	}
	return out, nil
}

func (s *snapshot) assetCode(id uuid.UUID) string {
	if asset, ok := s.meta.assets[id]; ok {
		return asset.Code
	}
	return ""
}

func (s *snapshot) balanceRow(row *sqlcgen.LatestBalanceProjection) (BalanceRow, error) {
	identityID, err := uuid.FromBytes(row.IdentityID)
	if err != nil {
		return BalanceRow{}, err
	}
	accountID, err := uuid.FromBytes(row.AccountID)
	if err != nil {
		return BalanceRow{}, err
	}
	listingID, err := uuid.FromBytes(row.AssetListingID)
	if err != nil {
		return BalanceRow{}, err
	}
	item := BalanceRow{
		IdentityID: identityID, AccountID: accountID, ListingID: listingID,
		Total: nullCompact(row.TotalQuantity), Available: nullCompact(row.AvailableQuantity), Locked: nullCompact(row.LockedQuantity),
		ObservedAt: row.SourceObservedAt, Freshness: row.FreshnessState, Completeness: row.Completeness,
	}
	if ident, ok := s.meta.identities[identityID]; ok {
		item.IdentityName = ident.DisplayName
		if platformID, err := uuid.FromBytes(ident.PlatformID); err == nil {
			item.PlatformID = platformID
			item.PlatformCode = s.meta.platforms[platformID].Code
		}
	}
	if account, ok := s.meta.accounts[accountID]; ok {
		item.AccountName = account.DisplayName
	}
	if listing, ok := s.meta.listings[listingID]; ok {
		item.ListingSymbol = listing.Symbol
	}
	if assetID := optionalID(row.AssetID); assetID != nil {
		item.AssetID = assetID
		item.AssetCode = s.assetCode(*assetID)
	}
	return item, nil
}

func (s *snapshot) positionRow(row *sqlcgen.LatestPositionProjection) (PositionRow, error) {
	identityID, err := uuid.FromBytes(row.IdentityID)
	if err != nil {
		return PositionRow{}, err
	}
	accountID, err := uuid.FromBytes(row.AccountID)
	if err != nil {
		return PositionRow{}, err
	}
	instrumentID, err := uuid.FromBytes(row.InstrumentID)
	if err != nil {
		return PositionRow{}, err
	}
	item := PositionRow{
		IdentityID: identityID, AccountID: accountID, InstrumentID: instrumentID,
		Ref: row.PlatformPositionRef, Side: row.Side, Quantity: compactDecimal(row.Quantity),
		Long: compactDecimal(row.UnderlyingLongQuantity), Short: compactDecimal(row.UnderlyingShortQuantity),
		Mark: nullCompact(row.SelectedMarkPrice), ObservedAt: row.SourceObservedAt,
		Freshness: row.FreshnessState, Completeness: row.Completeness,
	}
	if ident, ok := s.meta.identities[identityID]; ok {
		item.IdentityName = ident.DisplayName
		if platformID, err := uuid.FromBytes(ident.PlatformID); err == nil {
			item.PlatformID = platformID
			item.PlatformCode = s.meta.platforms[platformID].Code
		}
	}
	if account, ok := s.meta.accounts[accountID]; ok {
		item.AccountName = account.DisplayName
	}
	if inst, ok := s.meta.instruments[instrumentID]; ok {
		item.Symbol = inst.Symbol
	}
	return item, nil
}

func matchBalance(item *BalanceRow, in *ListFilter) bool {
	if in.IdentityID != nil && item.IdentityID != *in.IdentityID {
		return false
	}
	if in.AccountID != nil && item.AccountID != *in.AccountID {
		return false
	}
	if in.ListingID != nil && item.ListingID != *in.ListingID {
		return false
	}
	if in.AssetID != nil && (item.AssetID == nil || *item.AssetID != *in.AssetID) {
		return false
	}
	return true
}

func matchPosition(item *PositionRow, in *ListFilter) bool {
	if in.IdentityID != nil && item.IdentityID != *in.IdentityID {
		return false
	}
	if in.AccountID != nil && item.AccountID != *in.AccountID {
		return false
	}
	if in.InstrumentID != nil && item.InstrumentID != *in.InstrumentID {
		return false
	}
	return true
}

func groupBalances(items []BalanceRow, grouping string) []Group {
	if grouping == "" {
		grouping = "all"
	}
	buckets := map[string]*Group{}
	values := map[string]*groupSums{}
	for i := range items {
		item := &items[i]
		key, label := balanceGroupKey(item, grouping)
		grp, ok := buckets[key]
		if !ok {
			grp = &Group{Grouping: grouping, Key: key, Label: label, Completeness: "complete", Freshness: "fresh"}
			buckets[key] = grp
			values[key] = &groupSums{}
		}
		mergeStatus(&grp.Completeness, &grp.Freshness, item.Completeness, item.Freshness)
		addOptional(values[key].total(), item.Total)
		addOptional(values[key].available(), item.Available)
		addOptional(values[key].locked(), item.Locked)
	}
	out := make([]Group, 0, len(buckets))
	for key, grp := range buckets {
		sums := values[key]
		grp.Totals = GroupTotals{Total: sums.totalOut(), Available: sums.availableOut(), Locked: sums.lockedOut()}
		out = append(out, *grp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func groupPositions(items []PositionRow, grouping string) []Group {
	if grouping == "" {
		grouping = "all"
	}
	buckets := map[string]*Group{}
	values := map[string]*groupSums{}
	for i := range items {
		item := &items[i]
		key, label := positionGroupKey(item, grouping)
		grp, ok := buckets[key]
		if !ok {
			grp = &Group{Grouping: grouping, Key: key, Label: label, Completeness: "complete", Freshness: "fresh"}
			buckets[key] = grp
			values[key] = &groupSums{}
		}
		mergeStatus(&grp.Completeness, &grp.Freshness, item.Completeness, item.Freshness)
		addOptional(values[key].quantity(), &item.Quantity)
		addOptional(values[key].long(), &item.Long)
		addOptional(values[key].short(), &item.Short)
	}
	out := make([]Group, 0, len(buckets))
	for key, grp := range buckets {
		sums := values[key]
		grp.Totals = GroupTotals{Quantity: sums.quantityOut(), Long: sums.longOut(), Short: sums.shortOut()}
		out = append(out, *grp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func balanceGroupKey(item *BalanceRow, grouping string) (string, string) {
	switch grouping {
	case "platform":
		return item.PlatformID.String(), firstNonEmpty(item.PlatformCode, item.PlatformID.String())
	case "identity":
		return item.IdentityID.String(), firstNonEmpty(item.IdentityName, item.IdentityID.String())
	case "account":
		return item.AccountID.String(), firstNonEmpty(item.AccountName, item.AccountID.String())
	case "asset":
		if item.AssetID == nil {
			return "unmapped", "Unmapped"
		}
		return item.AssetID.String(), firstNonEmpty(item.AssetCode, item.AssetID.String())
	case "listing":
		return item.ListingID.String(), firstNonEmpty(item.ListingSymbol, item.ListingID.String())
	default:
		return "all", "All"
	}
}

func positionGroupKey(item *PositionRow, grouping string) (string, string) {
	switch grouping {
	case "platform":
		return item.PlatformID.String(), firstNonEmpty(item.PlatformCode, item.PlatformID.String())
	case "identity":
		return item.IdentityID.String(), firstNonEmpty(item.IdentityName, item.IdentityID.String())
	case "account":
		return item.AccountID.String(), firstNonEmpty(item.AccountName, item.AccountID.String())
	case "instrument":
		return item.InstrumentID.String(), firstNonEmpty(item.Symbol, item.InstrumentID.String())
	default:
		return "all", "All"
	}
}

type groupSums struct {
	totalSum, availableSum, lockedSum, quantitySum, longSum, shortSum *sumSlot
	longQtySum, shortQtySum, netQtySum                                *sumSlot
	longValSum, shortValSum, netValSum                                *sumSlot
}

type sumSlot struct {
	ok    bool
	value *string
}

func (g *groupSums) total() *sumSlot {
	if g.totalSum == nil {
		g.totalSum = &sumSlot{ok: true}
	}
	return g.totalSum
}

func (g *groupSums) available() *sumSlot {
	if g.availableSum == nil {
		g.availableSum = &sumSlot{ok: true}
	}
	return g.availableSum
}

func (g *groupSums) locked() *sumSlot {
	if g.lockedSum == nil {
		g.lockedSum = &sumSlot{ok: true}
	}
	return g.lockedSum
}

func (g *groupSums) quantity() *sumSlot {
	if g.quantitySum == nil {
		g.quantitySum = &sumSlot{ok: true}
	}
	return g.quantitySum
}

func (g *groupSums) long() *sumSlot {
	if g.longSum == nil {
		g.longSum = &sumSlot{ok: true}
	}
	return g.longSum
}

func (g *groupSums) short() *sumSlot {
	if g.shortSum == nil {
		g.shortSum = &sumSlot{ok: true}
	}
	return g.shortSum
}

func (g *groupSums) longQty() *sumSlot {
	if g.longQtySum == nil {
		g.longQtySum = &sumSlot{ok: true}
	}
	return g.longQtySum
}

func (g *groupSums) shortQty() *sumSlot {
	if g.shortQtySum == nil {
		g.shortQtySum = &sumSlot{ok: true}
	}
	return g.shortQtySum
}

func (g *groupSums) netQty() *sumSlot {
	if g.netQtySum == nil {
		g.netQtySum = &sumSlot{ok: true}
	}
	return g.netQtySum
}

func (g *groupSums) longVal() *sumSlot {
	if g.longValSum == nil {
		g.longValSum = &sumSlot{ok: true}
	}
	return g.longValSum
}

func (g *groupSums) shortVal() *sumSlot {
	if g.shortValSum == nil {
		g.shortValSum = &sumSlot{ok: true}
	}
	return g.shortValSum
}

func (g *groupSums) netVal() *sumSlot {
	if g.netValSum == nil {
		g.netValSum = &sumSlot{ok: true}
	}
	return g.netValSum
}

func (g *groupSums) totalOut() *string     { return slotOut(g.totalSum) }
func (g *groupSums) availableOut() *string { return slotOut(g.availableSum) }
func (g *groupSums) lockedOut() *string    { return slotOut(g.lockedSum) }
func (g *groupSums) quantityOut() *string  { return slotOut(g.quantitySum) }
func (g *groupSums) longOut() *string      { return slotOut(g.longSum) }
func (g *groupSums) shortOut() *string     { return slotOut(g.shortSum) }
func (g *groupSums) longQtyOut() *string   { return slotOut(g.longQtySum) }
func (g *groupSums) shortQtyOut() *string  { return slotOut(g.shortQtySum) }
func (g *groupSums) netQtyOut() *string    { return slotOut(g.netQtySum) }
func (g *groupSums) longValOut() *string   { return slotOut(g.longValSum) }
func (g *groupSums) shortValOut() *string  { return slotOut(g.shortValSum) }
func (g *groupSums) netValOut() *string    { return slotOut(g.netValSum) }

func slotOut(slot *sumSlot) *string {
	if slot == nil || !slot.ok {
		return nil
	}
	return slot.value
}

func addOptional(slot *sumSlot, value *string) {
	if !slot.ok {
		return
	}
	if value == nil {
		slot.ok = false
		slot.value = nil
		return
	}
	parsed, ok := parseRat(*value)
	if !ok {
		slot.ok = false
		slot.value = nil
		return
	}
	current := parseOrNil(ptrValue(slot.value))
	sum := formatRat(addRat(current, parsed))
	slot.value = &sum
}

func ptrValue(value *string) string {
	if value == nil {
		return "0"
	}
	return *value
}

func mergeStatus(completeness, freshness *string, rowComplete, rowFresh string) {
	*completeness = worseCompleteness(*completeness, rowComplete)
	*freshness = worseFresh(*freshness, rowFresh)
}

func worseCompleteness(a, b string) string {
	rank := map[string]int{"complete": 0, "partial": 1, "missing": 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func mergeRowContext(ctx *Context, items []BalanceRow) Context {
	if ctx == nil {
		return Context{}
	}
	out := *ctx
	if len(items) == 0 {
		return out
	}
	out.Completeness, out.Freshness, out.Coverage = "complete", "fresh", "complete"
	for i := range items {
		out.Completeness = worseCompleteness(out.Completeness, items[i].Completeness)
		out.Freshness = worseFresh(out.Freshness, items[i].Freshness)
		if items[i].AssetID == nil {
			out.Coverage = "partial"
		}
	}
	if out.Completeness == "missing" {
		out.Coverage = "missing"
	}
	return out
}

func mergePositionContext(ctx *Context, items []PositionRow) Context {
	if ctx == nil {
		return Context{}
	}
	out := *ctx
	if len(items) == 0 {
		return out
	}
	out.Completeness, out.Freshness, out.Coverage = "complete", "fresh", "complete"
	for i := range items {
		out.Completeness = worseCompleteness(out.Completeness, items[i].Completeness)
		out.Freshness = worseFresh(out.Freshness, items[i].Freshness)
	}
	return out
}

func latestValuation(rows []sqlcgen.PortfolioValuationProjection) *sqlcgen.PortfolioValuationProjection {
	var best *sqlcgen.PortfolioValuationProjection
	for i := range rows {
		if best == nil || rows[i].EvaluatedAt.After(best.EvaluatedAt) {
			best = &rows[i]
		}
	}
	return best
}

func amountFromValuation(row *sqlcgen.PortfolioValuationProjection) Amount {
	at := row.EvaluatedAt
	return Amount{
		Amount: nullCompact(row.Amount), Completeness: row.Completeness, Coverage: coverageFromCompleteness(row.Completeness),
		Freshness: row.FreshnessState, EvaluatedAt: &at,
	}
}

func capitalAt(rows []sqlcgen.ContributedCapitalProjection, at *time.Time) Amount {
	if len(rows) == 0 {
		return missingAmount()
	}
	var best *sqlcgen.ContributedCapitalProjection
	for i := range rows {
		if at != nil && rows[i].EvaluatedAt.After(*at) {
			continue
		}
		if best == nil || rows[i].EvaluatedAt.After(best.EvaluatedAt) {
			best = &rows[i]
		}
	}
	if best == nil {
		return missingAmount()
	}
	eval := best.EvaluatedAt
	return Amount{
		Amount: nullCompact(best.Amount), Completeness: best.Completeness, Coverage: coverageFromCompleteness(best.Completeness),
		Freshness: "fresh", EvaluatedAt: &eval,
	}
}

func currentMonthChange(rows []sqlcgen.ValueChangeProjection, cutoff time.Time) Amount {
	var best *sqlcgen.ValueChangeProjection
	for i := range rows {
		if rows[i].PeriodKind != "month" {
			continue
		}
		if !cutoff.IsZero() && rows[i].PeriodStartDate.After(cutoff) {
			continue
		}
		if best == nil || rows[i].PeriodStartDate.After(best.PeriodStartDate) {
			best = &rows[i]
		}
	}
	if best == nil {
		return missingAmount()
	}
	start, end := best.PeriodStartDate, best.PeriodEndDate
	return Amount{
		Amount: nullCompact(best.Amount), Completeness: completenessFromState(best.State), Coverage: coverageFromCompleteness(completenessFromState(best.State)),
		Freshness: "fresh", State: best.State, PeriodKind: best.PeriodKind, PeriodStartDate: &start, PeriodEndDate: &end,
	}
}

func completenessFromState(state string) string {
	switch state {
	case "complete", "provisional":
		return "complete"
	case "incomplete", "incomparable":
		return "partial"
	default:
		return "missing"
	}
}

func coverageFromCompleteness(value string) string {
	switch value {
	case "complete":
		return "complete"
	case "partial":
		return "partial"
	default:
		return "missing"
	}
}

func missingAmount() Amount {
	return Amount{Completeness: "missing", Coverage: "missing", Freshness: "unknown"}
}

func issuesFromRows(rows []sqlcgen.ProjectionIssue) []Issue {
	out := make([]Issue, 0, len(rows))
	for i := range rows {
		item := Issue{
			Kind: rows[i].ProjectionKind, Key: rows[i].ProjectionKey, Code: rows[i].Code,
			Severity: rows[i].Severity, Summary: rows[i].Summary, First: rows[i].FirstEffectiveAt, Last: rows[i].LastEffectiveAt,
		}
		if id, err := uuid.FromBytes(rows[i].ID); err == nil {
			item.ID = &id
		}
		item.IdentityID = optionalID(rows[i].IdentityID)
		item.AccountID = optionalID(rows[i].AccountID)
		item.ObservationID = optionalID(rows[i].ObservationID)
		out = append(out, item)
	}
	return out
}

func nullCompact(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	compacted := compactDecimal(value.String)
	return &compacted
}

func pageSlice[T any](items []T, offset, limit int) []T {
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func normalizePage(offset, limit int) (int, int, error) {
	if offset < 0 {
		return 0, 0, ErrInvalidPage
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maxLimit {
		return 0, 0, ErrInvalidPage
	}
	return offset, limit, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
