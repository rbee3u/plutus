package projection

import (
	"context"
	"math/big"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// ExposureRow is one grouped or asset-level exposure.
type ExposureRow struct {
	AssetID                                 uuid.UUID
	AssetCode                               string
	PlatformID, IdentityID, AccountID       *uuid.UUID
	PlatformCode, IdentityName, AccountName string
	Long, Short, Net                        string
	LongValue, ShortValue, NetValue         *string
	Completeness, Freshness                 string
	EvaluatedAt                             *time.Time
}

// ExposurePage is a filtered, grouped exposure response.
type ExposurePage struct {
	Items   []ExposureRow
	Groups  []Group
	Offset  int
	Limit   int
	Total   int
	Context Context
	Issues  []Issue
}

// ExposureFilter is the accepted exposures query.
type ExposureFilter struct {
	Offset, Limit                  int
	GroupBy                        string
	IdentityID, AccountID, AssetID *uuid.UUID
}

// SeriesFilter is the accepted time-series query.
type SeriesFilter struct {
	From, To    *time.Time
	Granularity string
	PeriodKind  string
	MaxPoints   int
	Limit       int
	Cursor      string
}

// MissingInterval is a gap in a sampled series.
type MissingInterval struct {
	Start, End time.Time
}

// ValuationPage is sampled valuation history.
type ValuationPage struct {
	Items            []ValuationPoint
	Limit            int
	NextCursor       *string
	HasMore          bool
	Context          Context
	Granularity      string
	MissingIntervals []MissingInterval
	Issues           []Issue
}

// ValueChangeRow is one calendar value-change projection.
type ValueChangeRow struct {
	ID                     uuid.UUID
	PeriodKind             string
	PeriodStart, PeriodEnd time.Time
	Amount                 *string
	State                  string
	ContributionAmount     *string
	WithdrawalAmount       *string
	CoverageFingerprint    string
}

// ValueChangePage is sampled value-change history.
type ValueChangePage struct {
	Items            []ValueChangeRow
	Limit            int
	NextCursor       *string
	HasMore          bool
	Context          Context
	Granularity      string
	MissingIntervals []MissingInterval
	Issues           []Issue
}

// PeriodSummaryRow is one calendar period summary.
type PeriodSummaryRow struct {
	PeriodKind                           string
	PeriodStart, PeriodEnd               time.Time
	AsOfAt                               time.Time
	ValuationOpen, ValuationClose        *string
	ValuationHigh, ValuationLow          *string
	ContributedCapitalClose, ValueChange *string
	Completeness, Freshness              string
}

// PeriodSummaryPage is period-summary history.
type PeriodSummaryPage struct {
	Items      []PeriodSummaryRow
	Limit      int
	NextCursor *string
	HasMore    bool
	Context    Context
	Issues     []Issue
}

// ListExposures returns long/short/net exposure with whitelist grouping.
func (s *Service) ListExposures(ctx context.Context, portfolioID uuid.UUID, in *ExposureFilter) (ExposurePage, error) {
	if in == nil {
		in = &ExposureFilter{}
	}
	if !exposureGroups[in.GroupBy] {
		return ExposurePage{}, ErrInvalidGroup
	}
	offset, limit, err := normalizePage(in.Offset, in.Limit)
	if err != nil {
		return ExposurePage{}, err
	}
	snap, err := s.loadSnapshot(ctx, portfolioID)
	if err != nil {
		return ExposurePage{}, err
	}
	page := ExposurePage{Offset: offset, Limit: limit, Context: snap.context, Issues: snap.issues, Groups: []Group{}}
	if snap.generationID == uuid.Nil {
		return page, nil
	}
	items, err := s.exposureItems(ctx, &snap, in)
	if err != nil {
		return ExposurePage{}, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].AssetID != items[j].AssetID {
			return items[i].AssetID.String() < items[j].AssetID.String()
		}
		return exposureRowKey(&items[i]) < exposureRowKey(&items[j])
	})
	page.Total = len(items)
	page.Groups = groupExposures(items, in.GroupBy)
	page.Items = pageSlice(items, offset, limit)
	page.Context = mergeExposureContext(&page.Context, items)
	return page, nil
}

func (s *Service) exposureItems(ctx context.Context, snap *snapshot, in *ExposureFilter) ([]ExposureRow, error) {
	dimensional := in.GroupBy == "platform" || in.GroupBy == "identity" || in.GroupBy == "account" || in.IdentityID != nil || in.AccountID != nil
	if dimensional {
		return s.dimensionalExposures(ctx, snap, in)
	}
	rows, err := s.q.ListExposureProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return nil, err
	}
	target := latestExposureTime(rows)
	items := make([]ExposureRow, 0, len(rows))
	for i := range rows {
		if !target.IsZero() && !rows[i].EvaluatedAt.Equal(target) {
			continue
		}
		item, err := snap.exposureFromStored(&rows[i])
		if err != nil {
			return nil, err
		}
		if in.AssetID != nil && item.AssetID != *in.AssetID {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) dimensionalExposures(ctx context.Context, snap *snapshot, in *ExposureFilter) ([]ExposureRow, error) {
	balances, err := s.q.ListLatestBalanceProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return nil, err
	}
	positions, err := s.q.ListLatestPositionProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return nil, err
	}
	type acc struct {
		long, short *big.Rat
		complete    bool
		fresh       string
		assetID     uuid.UUID
		identityID  uuid.UUID
		accountID   uuid.UUID
		platformID  uuid.UUID
	}
	buckets := map[string]*acc{}
	ensure := func(assetID, identityID, accountID uuid.UUID) *acc {
		key := accountID.String() + "/" + assetID.String()
		item, ok := buckets[key]
		if !ok {
			item = &acc{
				long: new(big.Rat), short: new(big.Rat), complete: true, fresh: "fresh",
				assetID: assetID, identityID: identityID, accountID: accountID,
			}
			if ident, ok := snap.meta.identities[identityID]; ok {
				if platformID, err := uuid.FromBytes(ident.PlatformID); err == nil {
					item.platformID = platformID
				}
			}
			buckets[key] = item
		}
		return item
	}
	for i := range balances {
		row, err := snap.balanceRow(&balances[i])
		if err != nil {
			return nil, err
		}
		if row.AssetID == nil {
			continue
		}
		qty := quantityPref(row.Total, row.Available, row.Locked)
		if qty == nil {
			continue
		}
		bucket := ensure(*row.AssetID, row.IdentityID, row.AccountID)
		if ratSign(qty) < 0 {
			bucket.short = addRat(bucket.short, absRat(qty))
		} else {
			bucket.long = addRat(bucket.long, qty)
		}
		if row.Freshness == "stale" {
			bucket.fresh = "stale"
		}
		if row.Completeness != "complete" {
			bucket.complete = false
		}
	}
	for i := range positions {
		row, err := snap.positionRow(&positions[i])
		if err != nil {
			return nil, err
		}
		assetID, ok := snap.instrumentAsset(row.InstrumentID)
		if !ok {
			continue
		}
		bucket := ensure(assetID, row.IdentityID, row.AccountID)
		bucket.long = addRat(bucket.long, parseRatMust(row.Long))
		bucket.short = addRat(bucket.short, parseRatMust(row.Short))
		if row.Freshness == "stale" {
			bucket.fresh = "stale"
		}
		if row.Completeness != "complete" {
			bucket.complete = false
		}
	}
	var evaluated *time.Time
	if !snap.cutoff.IsZero() {
		cutoff := snap.cutoff
		evaluated = &cutoff
	}
	items := make([]ExposureRow, 0, len(buckets))
	for _, item := range buckets {
		if in.IdentityID != nil && item.identityID != *in.IdentityID {
			continue
		}
		if in.AccountID != nil && item.accountID != *in.AccountID {
			continue
		}
		if in.AssetID != nil && item.assetID != *in.AssetID {
			continue
		}
		identityID, accountID, platformID := item.identityID, item.accountID, item.platformID
		net := subRat(item.long, item.short)
		out := ExposureRow{
			AssetID: item.assetID, AssetCode: snap.assetCode(item.assetID),
			IdentityID: &identityID, AccountID: &accountID,
			IdentityName: snap.meta.identities[identityID].DisplayName,
			AccountName:  snap.meta.accounts[accountID].DisplayName,
			Long:         formatRat(item.long), Short: formatRat(item.short), Net: formatRat(net),
			Freshness: item.fresh, Completeness: "complete", EvaluatedAt: evaluated,
		}
		if platformID != uuid.Nil {
			out.PlatformID = &platformID
			out.PlatformCode = snap.meta.platforms[platformID].Code
		}
		if !item.complete {
			out.Completeness = "partial"
		}
		items = append(items, out)
	}
	return items, nil
}

func (s *snapshot) exposureFromStored(row *sqlcgen.ExposureProjection) (ExposureRow, error) {
	assetID, err := uuid.FromBytes(row.AssetID)
	if err != nil {
		return ExposureRow{}, err
	}
	at := row.EvaluatedAt
	return ExposureRow{
		AssetID: assetID, AssetCode: s.assetCode(assetID),
		Long: compactDecimal(row.LongQuantity), Short: compactDecimal(row.ShortQuantity), Net: compactDecimal(row.NetQuantity),
		LongValue: nullCompact(row.LongValue), ShortValue: nullCompact(row.ShortValue), NetValue: nullCompact(row.NetValue),
		Completeness: row.Completeness, Freshness: row.FreshnessState, EvaluatedAt: &at,
	}, nil
}

func (s *snapshot) instrumentAsset(instrumentID uuid.UUID) (uuid.UUID, bool) {
	inst, ok := s.meta.instruments[instrumentID]
	if !ok {
		return uuid.Nil, false
	}
	for _, listing := range [][]byte{inst.UnderlyingListingID, inst.BaseListingID} {
		id := optionalID(listing)
		if id == nil {
			continue
		}
		if assetID, ok := s.meta.mappings[*id]; ok {
			return assetID, true
		}
	}
	return uuid.Nil, false
}

func latestExposureTime(rows []sqlcgen.ExposureProjection) time.Time {
	var best time.Time
	for i := range rows {
		if best.IsZero() || rows[i].EvaluatedAt.After(best) {
			best = rows[i].EvaluatedAt
		}
	}
	return best
}

func exposureRowKey(item *ExposureRow) string {
	if item.AccountID != nil {
		return item.AccountID.String()
	}
	if item.IdentityID != nil {
		return item.IdentityID.String()
	}
	if item.PlatformID != nil {
		return item.PlatformID.String()
	}
	return item.AssetID.String()
}

func groupExposures(items []ExposureRow, grouping string) []Group {
	if grouping == "" {
		grouping = "all"
	}
	buckets := map[string]*Group{}
	values := map[string]*groupSums{}
	assets := map[string]uuid.UUID{}
	mixed := map[string]bool{}
	for i := range items {
		item := &items[i]
		key, label := exposureGroupKey(item, grouping)
		grp, ok := buckets[key]
		if !ok {
			grp = &Group{Grouping: grouping, Key: key, Label: label, Completeness: "complete", Freshness: "fresh"}
			buckets[key] = grp
			values[key] = &groupSums{}
			assets[key] = item.AssetID
		} else if assets[key] != item.AssetID {
			mixed[key] = true
		}
		mergeStatus(&grp.Completeness, &grp.Freshness, item.Completeness, item.Freshness)
		if !mixed[key] {
			addOptional(values[key].longQty(), &item.Long)
			addOptional(values[key].shortQty(), &item.Short)
			addOptional(values[key].netQty(), &item.Net)
		}
		addOptional(values[key].longVal(), item.LongValue)
		addOptional(values[key].shortVal(), item.ShortValue)
		addOptional(values[key].netVal(), item.NetValue)
	}
	out := make([]Group, 0, len(buckets))
	for key, grp := range buckets {
		sums := values[key]
		grp.Totals = GroupTotals{LongValue: sums.longValOut(), ShortValue: sums.shortValOut(), NetValue: sums.netValOut()}
		if !mixed[key] {
			grp.Totals.LongQuantity = sums.longQtyOut()
			grp.Totals.ShortQuantity = sums.shortQtyOut()
			grp.Totals.NetQuantity = sums.netQtyOut()
		}
		out = append(out, *grp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func exposureGroupKey(item *ExposureRow, grouping string) (string, string) {
	switch grouping {
	case "platform":
		if item.PlatformID == nil {
			return "unmapped", "Unmapped"
		}
		return item.PlatformID.String(), firstNonEmpty(item.PlatformCode, item.PlatformID.String())
	case "identity":
		if item.IdentityID == nil {
			return "unmapped", "Unmapped"
		}
		return item.IdentityID.String(), firstNonEmpty(item.IdentityName, item.IdentityID.String())
	case "account":
		if item.AccountID == nil {
			return "unmapped", "Unmapped"
		}
		return item.AccountID.String(), firstNonEmpty(item.AccountName, item.AccountID.String())
	case "asset":
		return item.AssetID.String(), firstNonEmpty(item.AssetCode, item.AssetID.String())
	default:
		return "all", "All"
	}
}

func mergeExposureContext(ctx *Context, items []ExposureRow) Context {
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
	out.Coverage = coverageFromCompleteness(out.Completeness)
	return out
}

// ListValuations returns sampled portfolio valuation history.
func (s *Service) ListValuations(ctx context.Context, portfolioID uuid.UUID, in *SeriesFilter) (ValuationPage, error) {
	if in == nil {
		in = &SeriesFilter{}
	}
	page, snap, err := s.seriesSetup(ctx, portfolioID, in, valuationCursorKey)
	if err != nil {
		return ValuationPage{}, err
	}
	out := ValuationPage{Limit: page.limit, Context: snap.context, Issues: snap.issues}
	if snap.generationID == uuid.Nil {
		return out, nil
	}
	rows, err := s.q.ListPortfolioValuationProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return ValuationPage{}, err
	}
	points := make([]samplePoint, 0, len(rows))
	for i := range rows {
		if !inWindow(rows[i].EvaluatedAt, in.From, in.To) {
			continue
		}
		id, err := uuid.FromBytes(rows[i].ID)
		if err != nil {
			return ValuationPage{}, err
		}
		points = append(points, samplePoint{At: rows[i].EvaluatedAt, ID: id, Index: i})
	}
	sampled, grain := samplePoints(points, loadLocation(snap.context.Timezone), in.Granularity, page.maxPoints)
	out.Granularity = grain
	out.MissingIntervals = missingFromPoints(sampled, grain)
	items := make([]ValuationPoint, 0, len(sampled))
	ids := make([]uuid.UUID, 0, len(sampled))
	for _, point := range sampled {
		if page.cursor != nil && !afterCursor(point.At.UTC().UnixMicro(), point.ID, *page.cursor) {
			continue
		}
		row := &rows[point.Index]
		items = append(items, ValuationPoint{
			EvaluatedAt: row.EvaluatedAt, Amount: nullCompact(row.Amount), Completeness: row.Completeness,
		})
		ids = append(ids, point.ID)
	}
	out.HasMore = len(items) > page.limit
	if out.HasMore {
		items = items[:page.limit]
		ids = ids[:page.limit]
	}
	out.Items = items
	if out.HasMore && len(items) > 0 {
		encoded, err := encodeCursor(valuationCursorKey, listCursor{
			PortfolioID: portfolioID, At: items[len(items)-1].EvaluatedAt.UTC().UnixMicro(), ID: ids[len(ids)-1],
		})
		if err != nil {
			return ValuationPage{}, err
		}
		out.NextCursor = &encoded
	}
	return out, nil
}

// ListValueChanges returns sampled calendar value-change history.
func (s *Service) ListValueChanges(ctx context.Context, portfolioID uuid.UUID, in *SeriesFilter) (ValueChangePage, error) {
	if in == nil {
		in = &SeriesFilter{}
	}
	page, snap, err := s.seriesSetup(ctx, portfolioID, in, valueChangeCursorKey)
	if err != nil {
		return ValueChangePage{}, err
	}
	kind := in.PeriodKind
	if kind == "" {
		kind = "month"
	}
	out := ValueChangePage{Limit: page.limit, Context: snap.context, Issues: snap.issues}
	if snap.generationID == uuid.Nil {
		return out, nil
	}
	rows, err := s.q.ListValueChangeProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return ValueChangePage{}, err
	}
	matched := make([]sqlcgen.ValueChangeProjection, 0, len(rows))
	for i := range rows {
		if rows[i].PeriodKind != kind {
			continue
		}
		if !periodOverlaps(rows[i].PeriodStartDate, rows[i].PeriodEndDate, in.From, in.To) {
			continue
		}
		matched = append(matched, rows[i])
	}
	points := make([]samplePoint, 0, len(matched))
	for i := range matched {
		id, err := uuid.FromBytes(matched[i].ID)
		if err != nil {
			return ValueChangePage{}, err
		}
		points = append(points, samplePoint{At: matched[i].PeriodEndDate, ID: id, Index: i})
	}
	grain := in.Granularity
	if grainRank(grain) > 0 && grainRank(grain) <= grainRank(kind) {
		grain = "auto"
	}
	sampled, used := samplePoints(points, loadLocation(snap.context.Timezone), grain, page.maxPoints)
	out.Granularity = used
	out.MissingIntervals = missingFromPoints(sampled, used)
	items := make([]ValueChangeRow, 0, len(sampled))
	for _, point := range sampled {
		if page.cursor != nil && !afterCursor(point.At.UTC().UnixMicro(), point.ID, *page.cursor) {
			continue
		}
		row := &matched[point.Index]
		item, err := valueChangeFromRow(row)
		if err != nil {
			return ValueChangePage{}, err
		}
		items = append(items, item)
	}
	out.HasMore = len(items) > page.limit
	if out.HasMore {
		items = items[:page.limit]
	}
	out.Items = items
	if out.HasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := encodeCursor(valueChangeCursorKey, listCursor{
			PortfolioID: portfolioID, At: last.PeriodEnd.UTC().UnixMicro(), ID: last.ID,
		})
		if err != nil {
			return ValueChangePage{}, err
		}
		out.NextCursor = &encoded
	}
	return out, nil
}

// ListPeriodSummaries returns calendar period summaries.
func (s *Service) ListPeriodSummaries(ctx context.Context, portfolioID uuid.UUID, in *SeriesFilter) (PeriodSummaryPage, error) {
	if in == nil {
		in = &SeriesFilter{}
	}
	page, snap, err := s.seriesSetup(ctx, portfolioID, in, periodSummaryCursorKey)
	if err != nil {
		return PeriodSummaryPage{}, err
	}
	kind := in.PeriodKind
	if kind == "" {
		kind = "month"
	}
	out := PeriodSummaryPage{Limit: page.limit, Context: snap.context, Issues: snap.issues}
	if snap.generationID == uuid.Nil {
		return out, nil
	}
	rows, err := s.q.ListPeriodSummaryProjections(ctx, mustBin(snap.generationID))
	if err != nil {
		return PeriodSummaryPage{}, err
	}
	items := make([]PeriodSummaryRow, 0, len(rows))
	for i := range rows {
		if rows[i].PeriodKind != kind {
			continue
		}
		if !periodOverlaps(rows[i].PeriodStartDate, rows[i].PeriodEndDate, in.From, in.To) {
			continue
		}
		id := periodCursorID(rows[i].PeriodKind, rows[i].PeriodStartDate.UTC().UnixMicro(), rows[i].PeriodEndDate.UTC().UnixMicro())
		at := rows[i].PeriodEndDate.UTC().UnixMicro()
		if page.cursor != nil && !afterCursor(at, id, *page.cursor) {
			continue
		}
		items = append(items, periodSummaryFromRow(&rows[i]))
	}
	out.HasMore = len(items) > page.limit
	if out.HasMore {
		items = items[:page.limit]
	}
	out.Items = items
	if out.HasMore && len(items) > 0 {
		last := items[len(items)-1]
		encoded, err := encodeCursor(periodSummaryCursorKey, listCursor{
			PortfolioID: portfolioID,
			At:          last.PeriodEnd.UTC().UnixMicro(),
			ID:          periodCursorID(last.PeriodKind, last.PeriodStart.UTC().UnixMicro(), last.PeriodEnd.UTC().UnixMicro()),
		})
		if err != nil {
			return PeriodSummaryPage{}, err
		}
		out.NextCursor = &encoded
	}
	return out, nil
}

type seriesPage struct {
	limit, maxPoints int
	cursor           *listCursor
}

func (s *Service) seriesSetup(ctx context.Context, portfolioID uuid.UUID, in *SeriesFilter, cursorKey []byte) (seriesPage, snapshot, error) {
	if in.From != nil && in.To != nil && in.From.After(*in.To) {
		return seriesPage{}, snapshot{}, ErrInvalidQuery
	}
	if !granularities[in.Granularity] {
		return seriesPage{}, snapshot{}, ErrInvalidQuery
	}
	if in.PeriodKind != "" && !periodKinds[in.PeriodKind] {
		return seriesPage{}, snapshot{}, ErrInvalidQuery
	}
	limit, err := normalizeLimit(in.Limit)
	if err != nil {
		return seriesPage{}, snapshot{}, err
	}
	maxPoints, err := normalizeMaxPoints(in.MaxPoints)
	if err != nil {
		return seriesPage{}, snapshot{}, err
	}
	page := seriesPage{limit: limit, maxPoints: maxPoints}
	if in.Cursor != "" {
		decoded, err := decodeCursor(cursorKey, in.Cursor, portfolioID)
		if err != nil {
			return seriesPage{}, snapshot{}, err
		}
		page.cursor = &decoded
	}
	snap, err := s.loadSnapshot(ctx, portfolioID)
	if err != nil {
		return seriesPage{}, snapshot{}, err
	}
	return page, snap, nil
}

func valueChangeFromRow(row *sqlcgen.ValueChangeProjection) (ValueChangeRow, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return ValueChangeRow{}, err
	}
	return ValueChangeRow{
		ID: id, PeriodKind: row.PeriodKind, PeriodStart: row.PeriodStartDate, PeriodEnd: row.PeriodEndDate,
		Amount: nullCompact(row.Amount), State: row.State,
		ContributionAmount: nullCompact(row.ContributionAmount), WithdrawalAmount: nullCompact(row.WithdrawalAmount),
		CoverageFingerprint: row.CoverageFingerprint,
	}, nil
}

func periodSummaryFromRow(row *sqlcgen.PeriodSummaryProjection) PeriodSummaryRow {
	return PeriodSummaryRow{
		PeriodKind: row.PeriodKind, PeriodStart: row.PeriodStartDate, PeriodEnd: row.PeriodEndDate, AsOfAt: row.AsOfAt,
		ValuationOpen: nullCompact(row.ValuationOpen), ValuationClose: nullCompact(row.ValuationClose),
		ValuationHigh: nullCompact(row.ValuationHigh), ValuationLow: nullCompact(row.ValuationLow),
		ContributedCapitalClose: nullCompact(row.ContributedCapitalClose), ValueChange: nullCompact(row.ValueChange),
		Completeness: row.Completeness, Freshness: row.FreshnessState,
	}
}

func missingFromPoints(points []samplePoint, grain string) []MissingInterval {
	times := make([]time.Time, 0, len(points))
	for i := range points {
		times = append(times, points[i].At)
	}
	return missingIntervals(times, grain)
}

func normalizeLimit(limit int) (int, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maxLimit {
		return 0, ErrInvalidPage
	}
	return limit, nil
}
