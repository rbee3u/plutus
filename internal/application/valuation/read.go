package valuation

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"maps"
	"math/big"
	"slices"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapters"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// PointByCycle returns the point written by one durable Collection Cycle.
func (s *Service) PointByCycle(ctx context.Context, portfolioID, cycleID uuid.UUID) (Point, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (Point, error) {
		row, err := mysql.Queries(ctx, s.db).GetValuationPointByCycle(ctx, sqlcgen.GetValuationPointByCycleParams{PortfolioID: portfolioID[:], CycleID: cycleID[:]})
		if errors.Is(err, sql.ErrNoRows) {
			return Point{}, ErrNotFound
		}
		if err != nil {
			return Point{}, err
		}
		return s.PointWithEvents(ctx, (*sqlcgen.ValuationPoint)(&row))
	})
}

// Latest returns the newest point of a Portfolio.
func (s *Service) Latest(ctx context.Context, portfolioID uuid.UUID) (Point, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (Point, error) {
		row, err := mysql.Queries(ctx, s.db).GetLatestValuationPoint(ctx, portfolioID[:])
		if errors.Is(err, sql.ErrNoRows) {
			return Point{}, ErrNoPoint
		}
		if err != nil {
			return Point{}, err
		}
		return s.PointWithEvents(ctx, (*sqlcgen.ValuationPoint)(&row))
	})
}

// First returns the earliest point of the Portfolio.
func (s *Service) First(ctx context.Context, portfolioID uuid.UUID) (Point, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (Point, error) {
		row, err := mysql.Queries(ctx, s.db).GetFirstValuationPoint(ctx, sqlcgen.GetFirstValuationPointParams{PortfolioID: portfolioID[:]})
		if errors.Is(err, sql.ErrNoRows) {
			return Point{}, ErrNoPoint
		}
		if err != nil {
			return Point{}, err
		}
		return s.PointWithEvents(ctx, (*sqlcgen.ValuationPoint)(&row))
	})
}

// Get returns one point by its Collection Cycle ID.
func (s *Service) Get(ctx context.Context, portfolioID, cycleID uuid.UUID) (Point, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (Point, error) {
		row, err := mysql.Queries(ctx, s.db).GetValuationPoint(ctx, sqlcgen.GetValuationPointParams{PortfolioID: portfolioID[:], CycleID: cycleID[:]})
		if errors.Is(err, sql.ErrNoRows) {
			return Point{}, ErrNotFound
		}
		if err != nil {
			return Point{}, err
		}
		return s.PointWithEvents(ctx, (*sqlcgen.ValuationPoint)(&row))
	})
}

// Accounts returns the stored Account Valuations of a point.
func (s *Service) Accounts(ctx context.Context, portfolioID, cycleID uuid.UUID) ([]AccountValuation, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) ([]AccountValuation, error) {
		point, err := mysql.Queries(ctx, s.db).GetValuationPoint(ctx, sqlcgen.GetValuationPointParams{PortfolioID: portfolioID[:], CycleID: cycleID[:]})
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if point.DetailState == "expired" || point.DetailState == "pruning" {
			return nil, ErrDetailExpired
		}
		accounts, err := s.accountsByCycles(ctx, portfolioID, [][]byte{cycleID[:]})
		if err != nil {
			return nil, err
		}
		return accounts[cycleID], nil
	})
}

// accountsByCycles reads adopted evidence, never selecting replacement prices
// or sources. The caller bounds cycle IDs and owns the consistent read snapshot.
func (s *Service) accountsByCycles(ctx context.Context, portfolioID uuid.UUID, cycles [][]byte) (map[uuid.UUID][]AccountValuation, error) {
	out := make(map[uuid.UUID][]AccountValuation, len(cycles))
	for _, raw := range cycles {
		id, err := uuid.FromBytes(raw)
		if err != nil {
			return nil, err
		}
		out[id] = []AccountValuation{}
	}
	rows, err := mysql.Queries(ctx, s.db).ListReportsByCycles(ctx, sqlcgen.ListReportsByCyclesParams{PortfolioID: portfolioID[:], CycleIds: cycles})
	if err != nil {
		return nil, err
	}
	type key struct{ cycle, account uuid.UUID }
	indices := make(map[key]int, len(rows))
	for _, stored := range rows {
		row := sqlcgen.ListReportsByCycleRow(stored)
		item, err := reportFromRow(&row)
		if err != nil {
			return nil, err
		}
		cycle, err := uuid.FromBytes(row.CycleID)
		if err != nil {
			return nil, err
		}
		indices[key{cycle, item.AccountID}] = len(out[cycle])
		out[cycle] = append(out[cycle], item)
	}
	balances, err := mysql.Queries(ctx, s.db).ListBalanceValuationsByCycles(ctx, sqlcgen.ListBalanceValuationsByCyclesParams{PortfolioID: portfolioID[:], CycleIds: cycles})
	if err != nil {
		return nil, err
	}
	for i := range balances {
		line := &balances[i]
		cycle, err := uuid.FromBytes(line.CycleID)
		if err != nil {
			return nil, err
		}
		account, err := uuid.FromBytes(line.AccountID)
		if err != nil {
			return nil, err
		}
		index, ok := indices[key{cycle, account}]
		if !ok {
			return nil, errors.New("balance valuation has no Account Report")
		}
		if err := appendStoredBalance(&out[cycle][index], line); err != nil {
			return nil, err
		}
	}
	for _, accounts := range out {
		sortAccounts(accounts)
	}
	return out, nil
}

const valuationReadBatch = 256

func PointFromRow(row *sqlcgen.ValuationPoint) (Point, error) {
	if row.DetailState == "pruning" {
		copy := *row
		copy.DetailState = "expired"
		row = &copy
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Point{}, err
	}
	cycleID, err := uuid.FromBytes(row.CycleID)
	if err != nil {
		return Point{}, err
	}
	point := Point{
		PortfolioID:            portfolioID,
		CycleID:                cycleID,
		EvaluatedAt:            row.EvaluatedAt,
		TotalUSD:               decimal.Compact(row.TotalUsd),
		AccountCount:           int(row.AccountCount),
		StaleAccountCount:      int(row.StaleAccountCount),
		IncompleteAccountCount: int(row.IncompleteAccountCount),
		BoundaryEvents:         []BoundaryEvent{},
		Issues:                 []Issue{},
		CalculationVersion:     row.CalculationVersion,
		DetailState:            row.DetailState,
	}
	return point, nil
}

// PointWithEvents reads the derived details of one stored point.
func (s *Service) PointWithEvents(ctx context.Context, row *sqlcgen.ValuationPoint) (Point, error) {
	points, err := s.pointsWithEvents(ctx, []sqlcgen.ValuationPoint{*row}, true)
	if err != nil {
		return Point{}, err
	}
	return points[0], nil
}

// PointsWithEvents adds Boundary Events and comparison markers in bounded
// batches. Period comparisons do not need every Account's Balance evidence.
func (s *Service) PointsWithEvents(ctx context.Context, rows []sqlcgen.ValuationPoint) ([]Point, error) {
	return s.pointsWithEvents(ctx, rows, false)
}

func (s *Service) pointsWithEvents(ctx context.Context, rows []sqlcgen.ValuationPoint, includeAccountIssues bool) ([]Point, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) ([]Point, error) {
		out := make([]Point, 0, len(rows))
		for _, row := range rows {
			point, err := PointFromRow(&row)
			if err != nil {
				return nil, err
			}
			out = append(out, point)
		}
		if len(out) == 0 {
			return out, nil
		}
		previous, err := s.predecessors(ctx, out[0].PortfolioID, out)
		if err != nil {
			return nil, err
		}
		for start := 0; start < len(out); start += valuationReadBatch {
			batch := out[start:min(start+valuationReadBatch, len(out))]
			if err := s.hydratePointDetails(ctx, batch, previous, includeAccountIssues); err != nil {
				return nil, err
			}
		}
		return out, nil
	})
}

// hydratePointDetails derives every comparison marker except Manual
// adjustments from retained facts. Named Account omissions follow detail
// retention; an expired point keeps only its counts.
func (s *Service) hydratePointDetails(ctx context.Context, points []Point, previous map[uuid.UUID]sqlcgen.ListPointComparisonTimelineRow, includeAccountIssues bool) error {
	portfolio := points[0].PortfolioID
	cycles := make([][]byte, 0, len(points))
	retained := make([][]byte, 0, len(points))
	byCycle := make(map[uuid.UUID]*Point, len(points))
	for i := range points {
		point := &points[i]
		if point.PortfolioID != portfolio {
			return errors.New("point batch crosses Portfolios")
		}
		cycles = append(cycles, point.CycleID[:])
		if point.DetailState == "full" || point.DetailState == "daily" {
			retained = append(retained, point.CycleID[:])
		}
		byCycle[point.CycleID] = point
	}
	q := mysql.Queries(ctx, s.db)
	if includeAccountIssues && len(retained) > 0 {
		accounts, err := s.accountsByCycles(ctx, portfolio, retained)
		if err != nil {
			return err
		}
		for cycle, list := range accounts {
			for i := range list {
				byCycle[cycle].Issues = append(byCycle[cycle].Issues, AccountIssues(&list[i])...)
			}
		}
	}
	adjustments, err := q.ListManualBalanceChangesByCycles(ctx, sqlcgen.ListManualBalanceChangesByCyclesParams{PortfolioID: portfolio[:], CycleIds: cycles})
	if err != nil {
		return err
	}
	for _, row := range adjustments {
		cycle, err := uuid.FromBytes(row.CycleID)
		if err != nil {
			return err
		}
		account, err := uuid.FromBytes(row.AccountID)
		if err != nil {
			return err
		}
		adjustment := ManualBalanceChange{AccountID: account, AccountName: adapters.AccountName(adapters.AccountType(row.AccountType))}
		byCycle[cycle].ManualBalanceChanges = append(byCycle[cycle].ManualBalanceChanges, adjustment)
		byCycle[cycle].Issues = append(byCycle[cycle].Issues, adjustment.Issue())
	}
	mappings, err := q.ListAssetMappingChangesByCycles(ctx, sqlcgen.ListAssetMappingChangesByCyclesParams{PortfolioID: portfolio[:], CycleIds: cycles})
	if err != nil {
		return err
	}
	for _, row := range mappings {
		cycle, err := uuid.FromBytes(row.CycleID)
		if err != nil {
			return err
		}
		asset, err := uuid.FromBytes(row.AssetID)
		if err != nil {
			return err
		}
		byCycle[cycle].Issues = append(byCycle[cycle].Issues, Issue{Code: "asset_mapping_changed", AssetID: &asset, Symbol: row.Symbol, Detail: "The adopted mapping of " + row.Symbol + " changed; comparisons across this point include a change of valuation policy."})
	}
	for i := range points {
		point := &points[i]
		if prior, ok := previous[point.CycleID]; ok && prior.CalculationVersion != point.CalculationVersion {
			point.Issues = append(point.Issues, Issue{Code: "calculation_version_changed", Detail: "The valuation calculation changed; comparisons across this point include a change of valuation policy."})
		}
	}
	events, err := q.ListBoundaryEventsByCycles(ctx, sqlcgen.ListBoundaryEventsByCyclesParams{PortfolioID: portfolio[:], CycleIds: cycles})
	if err != nil {
		return err
	}
	totals := boundaryTotals{}
	for _, row := range events {
		cycle, err := uuid.FromBytes(row.CycleID)
		if err != nil {
			return err
		}
		point := byCycle[cycle]
		account, err := uuid.FromBytes(row.AccountID)
		if err != nil {
			return err
		}
		identity, err := uuid.FromBytes(row.IdentityID)
		if err != nil {
			return err
		}
		event := BoundaryEvent{
			CycleID: cycle, EvaluatedAt: point.EvaluatedAt,
			AccountID: account, IdentityID: identity, AccountName: adapters.AccountName(adapters.AccountType(row.AccountType)),
			IdentityName: row.IdentityName, Platform: row.Platform, Direction: row.Direction,
			Method: row.ValuationMethod, ValueUSD: nullDecimal(row.ValueUsd), ValuationComplete: row.ValuationComplete,
		}
		if event.ValueUSD != nil {
			event.valueRat = decimal.ParseOrZero(*event.ValueUSD)
		}
		totals.add(cycle, event.Direction, event.valueRat)
		point.BoundaryEvents = append(point.BoundaryEvents, event)
		if includeAccountIssues && !event.ValuationComplete {
			detail := "The account entered the boundary with an incomplete value."
			if event.Direction == DirectionWithdrawal {
				detail = "The account left the boundary with an incomplete value."
			}
			point.Issues = append(point.Issues, Issue{Code: "boundary_incomplete", AccountID: &event.AccountID, AccountName: event.AccountName, Detail: detail})
		}
	}
	for i := range points {
		points[i].BoundaryContributionUSD, points[i].BoundaryWithdrawalUSD = totals.of(points[i].CycleID)
	}
	return nil
}

// boundaryTotals sums known Boundary Event values per point and direction;
// an unvalued event contributes nothing.
type boundaryTotals map[uuid.UUID]*[2]*big.Rat

func (t boundaryTotals) add(cycle uuid.UUID, direction string, value *big.Rat) {
	if value == nil {
		return
	}
	sums := t[cycle]
	if sums == nil {
		sums = &[2]*big.Rat{decimal.Zero(), decimal.Zero()}
		t[cycle] = sums
	}
	index := 0
	if direction == DirectionWithdrawal {
		index = 1
	}
	sums[index] = decimal.Add(sums[index], value)
}

func (t boundaryTotals) of(cycle uuid.UUID) (contribution, withdrawal string) {
	sums := t[cycle]
	if sums == nil {
		zero := decimal.Format(decimal.Zero())
		return zero, zero
	}
	return decimal.Format(sums[0]), decimal.Format(sums[1])
}

// WithBoundaryTotals fills the Boundary totals of summary points without
// reading their events' Account labels.
func (s *Service) WithBoundaryTotals(ctx context.Context, points []Point) error {
	if len(points) == 0 {
		return nil
	}
	from, to := points[0].EvaluatedAt, points[0].EvaluatedAt
	for i := range points {
		from, to = minTime(from, points[i].EvaluatedAt), maxTime(to, points[i].EvaluatedAt)
	}
	totals, err := s.BoundaryTotalsBetween(ctx, points[0].PortfolioID, from, to)
	if err != nil {
		return err
	}
	for i := range points {
		points[i].BoundaryContributionUSD, points[i].BoundaryWithdrawalUSD = totals(points[i].CycleID)
	}
	return nil
}

// BoundaryTotalsBetween returns the contribution and withdrawal totals of
// every point evaluated within [from, to].
func (s *Service) BoundaryTotalsBetween(ctx context.Context, portfolioID uuid.UUID, from, to time.Time) (func(uuid.UUID) (contribution, withdrawal string), error) {
	rows, err := mysql.Queries(ctx, s.db).ListBoundaryValuesBetween(ctx, sqlcgen.ListBoundaryValuesBetweenParams{PortfolioID: portfolioID[:], FromAt: from.UTC(), ToAt: to.UTC()})
	if err != nil {
		return nil, err
	}
	totals := boundaryTotals{}
	for _, row := range rows {
		cycle, err := uuid.FromBytes(row.CycleID)
		if err != nil {
			return nil, err
		}
		if row.ValueUsd.Valid {
			totals.add(cycle, row.Direction, decimal.ParseOrZero(row.ValueUsd.String))
		}
	}
	return totals.of, nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// predecessors maps each point to its immediate predecessor in the Portfolio's
// timeline, which may fall outside the requested points.
func (s *Service) predecessors(ctx context.Context, portfolio uuid.UUID, points []Point) (map[uuid.UUID]sqlcgen.ListPointComparisonTimelineRow, error) {
	from, to := points[0].EvaluatedAt, points[0].EvaluatedAt
	selected := make(map[uuid.UUID]bool, len(points))
	for _, point := range points {
		selected[point.CycleID] = true
		from, to = minTime(from, point.EvaluatedAt), maxTime(to, point.EvaluatedAt)
	}
	timeline, err := mysql.Queries(ctx, s.db).ListPointComparisonTimeline(ctx, sqlcgen.ListPointComparisonTimelineParams{PortfolioID: portfolio[:], FromAt: from, ToAt: to})
	if err != nil {
		return nil, err
	}
	previous := make(map[uuid.UUID]sqlcgen.ListPointComparisonTimelineRow, len(points))
	for i := 1; i < len(timeline); i++ {
		cycle, err := uuid.FromBytes(timeline[i].CycleID)
		if err != nil {
			return nil, err
		}
		if selected[cycle] {
			previous[cycle] = timeline[i-1]
		}
	}
	return previous, nil
}

// manualBalanceChangesByPoints compares adopted quantities with each point's actual
// predecessor, which may fall outside the requested batch. Set comparison stays
// in Go to avoid nested correlated semijoins across the Portfolio's history.
func (s *Service) manualBalanceChangesByPoints(ctx context.Context, portfolio uuid.UUID, points []Point) (map[uuid.UUID][]ManualBalanceChange, error) {
	q := mysql.Queries(ctx, s.db)
	predecessors, err := s.predecessors(ctx, portfolio, points)
	if err != nil {
		return nil, err
	}
	previous := make(map[uuid.UUID]uuid.UUID, len(predecessors))
	requested := make(map[uuid.UUID]bool, len(predecessors)*2)
	for cycle, row := range predecessors {
		prior, err := uuid.FromBytes(row.CycleID)
		if err != nil {
			return nil, err
		}
		previous[cycle] = prior
		requested[cycle], requested[prior] = true, true
	}
	type accountInput struct {
		identity   uuid.UUID
		quantities map[uuid.UUID]string
	}
	type cycleInput struct {
		identities map[uuid.UUID]bool
		manual     map[uuid.UUID]accountInput
	}
	inputs := make(map[uuid.UUID]cycleInput, len(requested))
	ids := make([][]byte, 0, len(requested))
	for id := range requested {
		ids = append(ids, id[:])
	}
	for start := 0; start < len(ids); start += valuationReadBatch {
		rows, err := q.ListManualComparisonInputsByCycles(ctx, sqlcgen.ListManualComparisonInputsByCyclesParams{PortfolioID: portfolio[:], CycleIds: ids[start:min(start+valuationReadBatch, len(ids))]})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			cycle, err := uuid.FromBytes(row.CycleID)
			if err != nil {
				return nil, err
			}
			identity, err := uuid.FromBytes(row.IdentityID)
			if err != nil {
				return nil, err
			}
			input, exists := inputs[cycle]
			if !exists {
				input = cycleInput{identities: make(map[uuid.UUID]bool), manual: make(map[uuid.UUID]accountInput)}
				inputs[cycle] = input
			}
			input.identities[identity] = true
			if row.AccountType != string(adapters.TypeManual) || !row.ManualRevisionNumber.Valid {
				continue
			}
			account, err := uuid.FromBytes(row.AccountID)
			if err != nil {
				return nil, err
			}
			manual, exists := input.manual[account]
			if !exists {
				manual = accountInput{identity: identity, quantities: make(map[uuid.UUID]string)}
				input.manual[account] = manual
			}
			if row.Quantity.Valid {
				asset, err := uuid.FromBytes(row.AssetID)
				if err != nil {
					return nil, err
				}
				manual.quantities[asset] = decimal.Compact(row.Quantity.String)
			}
		}
	}
	out := make(map[uuid.UUID][]ManualBalanceChange)
	for cycle, prior := range previous {
		current, before := inputs[cycle], inputs[prior]
		for account, manual := range current.manual {
			if !before.identities[manual.identity] || maps.Equal(manual.quantities, before.manual[account].quantities) {
				continue
			}
			out[cycle] = append(out[cycle], ManualBalanceChange{AccountID: account, AccountName: adapters.AccountName(adapters.TypeManual)})
		}
		slices.SortFunc(out[cycle], func(a, b ManualBalanceChange) int { return bytes.Compare(a.AccountID[:], b.AccountID[:]) })
	}
	return out, nil
}
