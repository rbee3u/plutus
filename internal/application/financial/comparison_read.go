package financial

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// comparisonRead owns immutable inputs for every comparison in one request.
// Endpoint issues require account evidence; intermediate markers do not.
type comparisonRead struct {
	timeline []sqlcgen.ListPointComparisonTimelineRow
	boundary sql.NullTime
	points   map[uuid.UUID]*valuation.Point
	evidence []valuation.Point
	flows    []sqlcgen.CapitalFlow
	days     map[string]bool
	excluded map[uuid.UUID]map[uuid.UUID]bool
	assets   map[uuid.UUID]map[uuid.UUID]comparisonAsset
}

type comparisonAsset struct{ platform, symbol string }

func (r *comparisonRead) endpoint(at time.Time, inclusive bool) *valuation.Point {
	header := r.endpointHeader(at, inclusive)
	if header == nil {
		return nil
	}
	id, _ := uuid.FromBytes(header.CycleID) // Timeline IDs were decoded during hydration.
	return r.points[id]
}

func (r *comparisonRead) endpointHeader(at time.Time, inclusive bool) *sqlcgen.ListPointComparisonTimelineRow {
	if r.boundary.Valid && at.Before(r.boundary.Time) {
		return nil
	}
	i := sort.Search(len(r.timeline), func(i int) bool {
		return r.timeline[i].EvaluatedAt.After(at) || (!inclusive && r.timeline[i].EvaluatedAt.Equal(at))
	})
	if i == 0 {
		return nil
	}
	return &r.timeline[i-1]
}

func (s *Service) comparisonRead(ctx context.Context, p PortfolioContext, kind string, periods []period) (*comparisonRead, error) {
	r := &comparisonRead{points: map[uuid.UUID]*valuation.Point{}, days: map[string]bool{}, excluded: map[uuid.UUID]map[uuid.UUID]bool{}, assets: map[uuid.UUID]map[uuid.UUID]comparisonAsset{}}
	q := mysql.Queries(ctx, s.db)
	var err error
	r.boundary, err = q.GetPortfolioHistoryBoundary(ctx, p.ID[:])
	if err != nil {
		return nil, err
	}
	from, to := periods[0].start.UTC(), periods[len(periods)-1].end.UTC()
	if kind == PeriodCustom {
		r.timeline, err = s.comparisonEndpointTimeline(ctx, p, periods[0])
	} else {
		r.timeline, err = q.ListPointComparisonTimeline(ctx, sqlcgen.ListPointComparisonTimelineParams{PortfolioID: p.ID[:], FromAt: from, ToAt: to})
	}
	if err != nil {
		return nil, err
	}
	// The first anchor can itself be an endpoint, so retain its predecessor too.
	if kind != PeriodCustom && len(r.timeline) > 0 && r.timeline[0].EvaluatedAt.Before(from) {
		first := r.timeline[0]
		prior, err := q.GetValuationPointBefore(ctx, sqlcgen.GetValuationPointBeforeParams{PortfolioID: p.ID[:], At: first.EvaluatedAt, CycleID: first.CycleID})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			r.timeline = append([]sqlcgen.ListPointComparisonTimelineRow{{CycleID: prior.CycleID, EvaluatedAt: prior.EvaluatedAt, CalculationVersion: prior.CalculationVersion}}, r.timeline...)
		}
	}
	selected := map[string]bool{}
	comparable := false
	for _, per := range periods {
		start, end := r.endpointHeader(per.start, kind == PeriodCustom), r.endpointHeader(per.end, kind == PeriodCustom)
		for _, header := range []*sqlcgen.ListPointComparisonTimelineRow{start, end} {
			if header != nil {
				selected[string(header.CycleID)] = true
			}
		}
		comparable = comparable || (start != nil && end != nil && end.EvaluatedAt.After(start.EvaluatedAt))
	}
	ids := [][]byte{}
	loc := p.location()
	for _, header := range r.timeline {
		if selected[string(header.CycleID)] {
			ids = append(ids, header.CycleID)
		}
		if !r.boundary.Valid || !header.EvaluatedAt.Before(r.boundary.Time) {
			r.days[header.EvaluatedAt.In(loc).Format("2006-01-02")] = true
		}
	}
	rows, err := s.pointRowsByIDs(ctx, p.ID, ids)
	if err != nil {
		return nil, err
	}
	points, _, err := s.valuation.PointsWithAccounts(ctx, rows, r.timeline)
	if err != nil {
		return nil, err
	}
	for i := range points {
		r.points[points[i].CycleID] = &points[i]
	}
	if !comparable {
		return r, nil
	}
	from, to = points[0].EvaluatedAt.Add(time.Microsecond), points[len(points)-1].EvaluatedAt
	r.flows, err = q.ListCapitalFlows(ctx, sqlcgen.ListCapitalFlowsParams{PortfolioID: p.ID[:], FromAt: from, ToAt: to.Add(time.Microsecond)})
	if err != nil {
		return nil, err
	}
	headers, err := q.ListComparisonPointHeadersBetween(ctx, sqlcgen.ListComparisonPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: from, ToAt: to})
	if err != nil {
		return nil, err
	}
	if kind == PeriodCustom {
		versions, err := q.ListCalculationVersionChangesBetween(ctx, sqlcgen.ListCalculationVersionChangesBetweenParams{PortfolioID: p.ID[:], FromAt: from, ToAt: to})
		if err != nil {
			return nil, err
		}
		for _, h := range versions {
			headers = append(headers, sqlcgen.ListComparisonPointHeadersBetweenRow(h))
		}
	} else {
		for i := 1; i < len(r.timeline); i++ {
			h := r.timeline[i]
			if !h.EvaluatedAt.Before(from) && !h.EvaluatedAt.After(to) && h.CalculationVersion != r.timeline[i-1].CalculationVersion {
				headers = append(headers, sqlcgen.ListComparisonPointHeadersBetweenRow{CycleID: h.CycleID, EvaluatedAt: h.EvaluatedAt})
			}
		}
	}
	slices.SortFunc(headers, func(a, b sqlcgen.ListComparisonPointHeadersBetweenRow) int {
		if order := a.EvaluatedAt.Compare(b.EvaluatedAt); order != 0 {
			return order
		}
		return bytes.Compare(a.CycleID, b.CycleID)
	})
	headers = slices.CompactFunc(headers, func(a, b sqlcgen.ListComparisonPointHeadersBetweenRow) bool { return bytes.Equal(a.CycleID, b.CycleID) })
	otherIDs := [][]byte{}
	for _, h := range headers {
		if !selected[string(h.CycleID)] {
			otherIDs = append(otherIDs, h.CycleID)
		}
	}
	otherRows, err := s.pointRowsByIDs(ctx, p.ID, otherIDs)
	if err != nil {
		return nil, err
	}
	timeline := r.timeline
	if kind == PeriodCustom {
		timeline = nil
	}
	others, err := s.valuation.PointsWithTimeline(ctx, otherRows, timeline)
	if err != nil {
		return nil, err
	}
	byCycle := map[string]valuation.Point{}
	for _, point := range points {
		byCycle[string(point.CycleID[:])] = point
	}
	for _, point := range others {
		byCycle[string(point.CycleID[:])] = point
	}
	for _, h := range headers {
		r.evidence = append(r.evidence, byCycle[string(h.CycleID)])
	}
	if err := s.comparisonPolicies(ctx, p, r, points); err != nil {
		return nil, err
	}
	return r, nil
}

// Custom comparisons need only two endpoint seeks. Reading every header would
// transfer a ten-year timeline merely to compare two summaries.
func (s *Service) comparisonEndpointTimeline(ctx context.Context, p PortfolioContext, per period) ([]sqlcgen.ListPointComparisonTimelineRow, error) {
	q := mysql.Queries(ctx, s.db)
	timeline := []sqlcgen.ListPointComparisonTimelineRow{}
	seen := map[string]bool{}
	add := func(row sqlcgen.ValuationPoint) {
		if !seen[string(row.CycleID)] {
			seen[string(row.CycleID)] = true
			timeline = append(timeline, sqlcgen.ListPointComparisonTimelineRow{CycleID: row.CycleID, EvaluatedAt: row.EvaluatedAt, CalculationVersion: row.CalculationVersion})
		}
	}
	for _, at := range []time.Time{per.start, per.end} {
		row, err := q.GetValuationPointAtOrBefore(ctx, sqlcgen.GetValuationPointAtOrBeforeParams{PortfolioID: p.ID[:], At: sql.NullTime{Time: at.UTC(), Valid: true}})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		add(row)
		prior, err := q.GetValuationPointBefore(ctx, sqlcgen.GetValuationPointBeforeParams{PortfolioID: p.ID[:], At: row.EvaluatedAt, CycleID: row.CycleID})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			add(sqlcgen.ValuationPoint(prior))
		}
	}
	slices.SortFunc(timeline, func(a, b sqlcgen.ListPointComparisonTimelineRow) int {
		if order := a.EvaluatedAt.Compare(b.EvaluatedAt); order != 0 {
			return order
		}
		return bytes.Compare(a.CycleID, b.CycleID)
	})
	return timeline, nil
}

func (s *Service) comparisonPolicies(ctx context.Context, p PortfolioContext, r *comparisonRead, points []valuation.Point) error {
	q := mysql.Queries(ctx, s.db)
	var times []sqlcgen.ListCollectionCycleTimesRow
	for start := 0; start < len(points); start += 256 {
		ids := [][]byte{}
		for _, point := range points[start:min(start+256, len(points))] {
			ids = append(ids, point.CycleID[:])
		}
		batch, err := q.ListCollectionCycleTimes(ctx, sqlcgen.ListCollectionCycleTimesParams{PortfolioID: p.ID[:], CycleIds: ids})
		if err != nil {
			return err
		}
		times = append(times, batch...)
	}
	if len(times) == 0 {
		return nil
	}
	slices.SortFunc(times, func(a, b sqlcgen.ListCollectionCycleTimesRow) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})
	versions, err := q.ListAssetExclusionsForWindow(ctx, sqlcgen.ListAssetExclusionsForWindowParams{
		PortfolioID: p.ID[:], FromAt: times[0].CreatedAt, ToAt: times[len(times)-1].CreatedAt,
	})
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return nil
	}
	active, adopted := map[uuid.UUID]bool{}, map[uuid.UUID]uint64{}
	versionCursor := 0
	for _, cycle := range times {
		id, err := uuid.FromBytes(cycle.ID)
		if err != nil {
			return err
		}
		for versionCursor < len(versions) && versions[versionCursor].EffectiveAt.Before(cycle.CreatedAt) {
			v := versions[versionCursor]
			versionCursor++
			asset, err := uuid.FromBytes(v.AssetID)
			if err != nil {
				return err
			}
			// A later effective time can belong to an older version.
			if v.Version <= adopted[asset] {
				continue
			}
			adopted[asset] = v.Version
			if v.Excluded {
				active[asset] = true
			} else {
				delete(active, asset)
			}
		}
		r.excluded[id] = maps.Clone(active)
	}
	// Only assets whose endpoint policy differs can produce exclusion issues.
	changed := map[uuid.UUID]bool{}
	baseline := r.excluded[points[0].CycleID]
	for _, point := range points[1:] {
		active := r.excluded[point.CycleID]
		for asset := range baseline {
			if !active[asset] {
				changed[asset] = true
			}
		}
		for asset := range active {
			if !baseline[asset] {
				changed[asset] = true
			}
		}
	}
	if len(changed) == 0 {
		return nil
	}
	first := points[0]
	ids := make([][]byte, 0, len(changed))
	for id := range changed {
		ids = append(ids, id[:])
	}
	present := map[uuid.UUID]comparisonAsset{}
	var changes []sqlcgen.ListCalculatedAssetChangesBetweenRow
	for start := 0; start < len(ids); start += 256 {
		batch := ids[start:min(start+256, len(ids))]
		initial, err := q.ListCalculatedAssetsAtCycles(ctx, sqlcgen.ListCalculatedAssetsAtCyclesParams{PortfolioID: p.ID[:], StartCycleID: first.CycleID[:], EndCycleID: first.CycleID[:], AssetIds: batch})
		if err != nil {
			return err
		}
		for _, row := range initial {
			id, err := uuid.FromBytes(row.ID)
			if err != nil {
				return err
			}
			present[id] = comparisonAsset{row.Platform, row.Symbol}
		}
		rows, err := q.ListCalculatedAssetChangesBetween(ctx, sqlcgen.ListCalculatedAssetChangesBetweenParams{PortfolioID: p.ID[:], FromAt: first.EvaluatedAt, ToAt: points[len(points)-1].EvaluatedAt, AssetIds: batch})
		if err != nil {
			return err
		}
		changes = append(changes, rows...)
	}
	slices.SortFunc(changes, func(a, b sqlcgen.ListCalculatedAssetChangesBetweenRow) int {
		if order := a.EvaluatedAt.Compare(b.EvaluatedAt); order != 0 {
			return order
		}
		return bytes.Compare(a.CycleID, b.CycleID)
	})
	cursor := 0
	for _, point := range points {
		for cursor < len(changes) {
			c := changes[cursor]
			if c.EvaluatedAt.After(point.EvaluatedAt) || (c.EvaluatedAt.Equal(point.EvaluatedAt) && bytes.Compare(c.CycleID, point.CycleID[:]) > 0) {
				break
			}
			cursor++
			if c.EvaluatedAt.Equal(first.EvaluatedAt) && bytes.Compare(c.CycleID, first.CycleID[:]) <= 0 {
				continue
			}
			id, err := uuid.FromBytes(c.AssetID)
			if err != nil {
				return err
			}
			if c.Treatment == "absent" {
				delete(present, id)
			} else {
				present[id] = comparisonAsset{c.Platform, c.Symbol}
			}
		}
		r.assets[point.CycleID] = maps.Clone(present)
	}
	return nil
}

func (r *comparisonRead) exclusionIssues(start, end uuid.UUID) []valuation.Issue {
	assets := maps.Clone(r.assets[start])
	if assets == nil {
		assets = map[uuid.UUID]comparisonAsset{}
	}
	maps.Copy(assets, r.assets[end])
	changed := []comparisonAsset{}
	for id, asset := range assets {
		if r.excluded[start][id] != r.excluded[end][id] {
			changed = append(changed, asset)
		}
	}
	slices.SortFunc(changed, func(a, b comparisonAsset) int {
		if order := strings.Compare(a.platform, b.platform); order != 0 {
			return order
		}
		return strings.Compare(a.symbol, b.symbol)
	})
	issues := []valuation.Issue{}
	for _, asset := range changed {
		issues = append(issues, valuation.Issue{Code: "asset_exclusion_changed", Symbol: asset.symbol, Detail: asset.platform + " " + asset.symbol})
	}
	return issues
}
