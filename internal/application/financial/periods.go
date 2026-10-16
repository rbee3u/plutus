package financial

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/assetexclusion"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

const (
	PeriodDay     = "day"
	PeriodWeek    = "week"
	PeriodMonth   = "month"
	PeriodQuarter = "quarter"
	PeriodYear    = "year"
	PeriodCustom  = "custom"

	maxPeriods = 400
)

type period struct {
	label string
	start time.Time
	end   time.Time
}

// periodsBetween includes whole local periods intersecting [from, to).
func periodsBetween(kind string, from, to time.Time, loc *time.Location) ([]period, error) {
	if !to.After(from) {
		return nil, ErrInvalid
	}
	start := periodStart(kind, from.In(loc))
	if start.IsZero() {
		return nil, ErrInvalid
	}
	var out []period
	for cursor := start; cursor.Before(to); {
		next := periodNext(kind, cursor)
		out = append(out, period{label: periodLabel(kind, cursor), start: cursor, end: next})
		if len(out) > maxPeriods {
			return nil, fmt.Errorf("%w: at most 400 periods are supported; narrow the time range or select a larger period", ErrInvalid)
		}
		cursor = next
	}
	return out, nil
}

// Calendar rows start with the first retained point's whole local period.
// Use portfolio history rather than the query's first point to preserve gaps.
func (s *Service) periodsInHistory(ctx context.Context, p PortfolioContext, kind string, from, to time.Time) ([]period, time.Time, error) {
	loc := p.location()
	if !to.After(from) || periodStart(kind, from.In(loc)).IsZero() {
		return nil, time.Time{}, ErrInvalid
	}
	first, err := mysql.Queries(ctx, s.db).GetFirstValuationPoint(ctx, sqlcgen.GetFirstValuationPointParams{PortfolioID: p.ID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return []period{}, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	start := periodStart(kind, first.EvaluatedAt.In(loc))
	if !to.After(start) {
		return []period{}, first.EvaluatedAt, nil
	}
	if from.Before(start) {
		from = start
	}
	periods, err := periodsBetween(kind, from, to, loc)
	return periods, first.EvaluatedAt, err
}

func periodStart(kind string, t time.Time) time.Time {
	loc := t.Location()
	switch kind {
	case PeriodDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	case PeriodWeek:
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		offset := (int(day.Weekday()) + 6) % 7 // ISO week starts on Monday
		return day.AddDate(0, 0, -offset)
	case PeriodMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	case PeriodQuarter:
		month := (int(t.Month())-1)/3*3 + 1
		return time.Date(t.Year(), time.Month(month), 1, 0, 0, 0, 0, loc)
	case PeriodYear:
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, loc)
	}
	return time.Time{}
}

func periodNext(kind string, start time.Time) time.Time {
	switch kind {
	case PeriodDay:
		return start.AddDate(0, 0, 1)
	case PeriodWeek:
		return start.AddDate(0, 0, 7)
	case PeriodMonth:
		return start.AddDate(0, 1, 0)
	case PeriodQuarter:
		return start.AddDate(0, 3, 0)
	default:
		return start.AddDate(1, 0, 0)
	}
}

func periodLabel(kind string, start time.Time) string {
	switch kind {
	case PeriodDay:
		return start.Format("2006-01-02")
	case PeriodWeek:
		year, week := start.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case PeriodMonth:
		return start.Format("2006-01")
	case PeriodQuarter:
		return fmt.Sprintf("%d-Q%d", start.Year(), (int(start.Month())-1)/3+1)
	default:
		return start.Format("2006")
	}
}

// PeriodCloses returns the period close of every period of kind between two instants.
func (s *Service) PeriodCloses(ctx context.Context, p PortfolioContext, kind string, from, to time.Time) ([]PeriodClose, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) ([]PeriodClose, error) {
		loc := p.location()
		periods, historyStart, err := s.periodsInHistory(ctx, p, kind, from, to)
		if err != nil {
			return nil, err
		}
		if len(periods) == 0 {
			return []PeriodClose{}, nil
		}
		headers, err := mysql.Queries(ctx, s.db).ListValuationPointHeadersBetween(ctx, sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: periods[0].start.UTC(), ToAt: periods[len(periods)-1].end.UTC()})
		if err != nil {
			return nil, err
		}
		ids := make([][]byte, 0, len(periods))
		selected := make([]int, len(periods))
		cursor := 0
		for i, per := range periods {
			selected[i] = -1
			for cursor < len(headers) && headers[cursor].EvaluatedAt.Before(per.end) {
				cursor++
			}
			if cursor > 0 && !headers[cursor-1].EvaluatedAt.Before(per.start) {
				selected[i] = len(ids)
				ids = append(ids, headers[cursor-1].CycleID)
			}
		}
		rows, err := s.pointRowsByIDs(ctx, p.ID, ids)
		if err != nil {
			return nil, err
		}
		points, err := s.pointsFromRows(ctx, rows)
		if err != nil {
			return nil, err
		}
		seen := observedDays(headers, loc)
		out := make([]PeriodClose, 0, len(periods))
		for i, per := range periods {
			item := PeriodClose{PeriodKind: kind, Label: per.label, PeriodStart: per.start, PeriodEnd: per.end, Completeness: valuation.CompletenessComplete, Issues: []valuation.Issue{}}
			var point *valuation.Point
			if selected[i] >= 0 {
				point = &points[selected[i]]
			}
			if point == nil {
				item.Completeness = valuation.CompletenessIncomplete
				item.Issues = append(item.Issues, valuation.Issue{Code: "no_point_in_period", Detail: "No valuation point exists in this period."})
			} else {
				item.Point = point
				if point.Completeness() == valuation.CompletenessIncomplete {
					item.Completeness = valuation.CompletenessIncomplete
					item.Issues = append(item.Issues, valuation.Issue{Code: "point_incomplete", Detail: "The closing valuation point is incomplete."})
				}
				if kind != PeriodDay {
					coverageStart := per.start
					if coverageStart.Before(historyStart) {
						coverageStart = historyStart
					}
					if missing := daysWithoutPoints(seen, coverageStart, per.end, loc); len(missing) > 0 {
						item.Completeness = valuation.CompletenessIncomplete
						item.Issues = append(item.Issues, valuation.Issue{Code: "days_without_points", Detail: joinDays(missing)})
					}
				}
			}
			out = append(out, item)
		}
		return out, nil
	})
}

// ValueChanges returns the value change of every period of kind between two instants.
func (s *Service) ValueChanges(ctx context.Context, p PortfolioContext, kind string, from, to time.Time) ([]ValueChange, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) ([]ValueChange, error) {
		periods, _, err := s.periodsInHistory(ctx, p, kind, from, to)
		if err != nil {
			return nil, err
		}
		if len(periods) == 0 {
			return []ValueChange{}, nil
		}
		out := make([]ValueChange, 0, len(periods))
		for _, per := range periods {
			change, err := s.valueChange(ctx, p, kind, per.label, per.start, per.end)
			if err != nil {
				return nil, err
			}
			out = append(out, change)
		}
		return out, nil
	})
}

// CustomValueChange returns one value change between the last points at or before two instants.
func (s *Service) CustomValueChange(ctx context.Context, p PortfolioContext, from, to time.Time) (ValueChange, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (ValueChange, error) {
		if !to.After(from) {
			return ValueChange{}, ErrInvalid
		}
		return s.valueChange(ctx, p, PeriodCustom, "custom", from, to)
	})
}

// Calendar endpoints use strictly earlier points; custom endpoints also accept
// a point at the requested instant. Flows follow the selected points' interval.
func (s *Service) valueChange(ctx context.Context, p PortfolioContext, kind, label string, periodStart, periodEnd time.Time) (ValueChange, error) {
	change := ValueChange{PeriodKind: kind, Label: label, From: periodStart, To: periodEnd, Completeness: valuation.CompletenessComplete, Issues: []valuation.Issue{}, BoundaryEvents: []valuation.BoundaryEvent{}}
	change.FlowsCompleteness = valuation.CompletenessIncomplete
	startPoint, err := s.pointAtOrBefore(ctx, p.ID, periodStart, kind == PeriodCustom)
	if err != nil {
		return ValueChange{}, err
	}
	endPoint, err := s.pointAtOrBefore(ctx, p.ID, periodEnd, kind == PeriodCustom)
	if err != nil {
		return ValueChange{}, err
	}
	change.StartPoint, change.EndPoint = startPoint, endPoint
	if startPoint == nil {
		change.Completeness = valuation.CompletenessIncomplete
		change.Issues = append(change.Issues, valuation.Issue{Code: "no_start_point", Detail: "No valuation point exists at the start endpoint."})
	}
	if endPoint == nil {
		change.Completeness = valuation.CompletenessIncomplete
		change.Issues = append(change.Issues, valuation.Issue{Code: "no_end_point", Detail: "No valuation point exists at the end endpoint."})
	}
	if startPoint == nil || endPoint == nil {
		return change, nil
	}
	if !endPoint.EvaluatedAt.After(startPoint.EvaluatedAt) {
		change.Completeness = valuation.CompletenessIncomplete
		change.Issues = append(change.Issues, valuation.Issue{Code: "no_point_in_period", Detail: "Two distinct valuation points are required for comparison."})
		return change, nil
	}
	change.Comparable = true
	change.FlowsCompleteness = valuation.CompletenessComplete
	if startPoint.Completeness() == valuation.CompletenessIncomplete {
		change.Completeness = valuation.CompletenessIncomplete
		change.Issues = append(change.Issues, valuation.Issue{Code: "start_point_incomplete", Detail: "The start valuation point is incomplete."})
	}
	if endPoint.Completeness() == valuation.CompletenessIncomplete {
		change.Completeness = valuation.CompletenessIncomplete
		change.Issues = append(change.Issues, valuation.Issue{Code: "end_point_incomplete", Detail: "The end valuation point is incomplete."})
	}
	if kind != PeriodCustom && kind != PeriodDay {
		if headers, err := mysql.Queries(ctx, s.db).ListValuationPointHeadersBetween(ctx, sqlcgen.ListValuationPointHeadersBetweenParams{PortfolioID: p.ID[:], FromAt: periodStart.UTC(), ToAt: periodEnd.UTC()}); err != nil {
			return ValueChange{}, err
		} else if missing := daysWithoutPoints(observedDays(headers, p.location()), periodStart, periodEnd, p.location()); len(missing) > 0 {
			change.Completeness = valuation.CompletenessIncomplete
			change.Issues = append(change.Issues, valuation.Issue{Code: "days_without_points", Detail: joinDays(missing)})
		}
	}
	policyIssues, err := s.exclusionChangeIssues(ctx, p.ID, startPoint.CycleID, endPoint.CycleID)
	if err != nil {
		return ValueChange{}, err
	}
	if len(policyIssues) > 0 {
		change.Completeness = valuation.CompletenessIncomplete
		change.Issues = append(change.Issues, policyIssues...)
	}
	contributions, withdrawals := decimal.Zero(), decimal.Zero()
	flows, err := mysql.Queries(ctx, s.db).ListCapitalFlows(ctx, sqlcgen.ListCapitalFlowsParams{PortfolioID: p.ID[:], FromAt: startPoint.EvaluatedAt.Add(time.Microsecond), ToAt: endPoint.EvaluatedAt.Add(time.Microsecond)})
	if err != nil {
		return ValueChange{}, err
	}
	for i := range flows {
		if !flows[i].ValueUsd.Valid {
			change.Completeness = valuation.CompletenessIncomplete
			change.FlowsCompleteness = valuation.CompletenessIncomplete
			flowID, err := uuid.FromBytes(flows[i].ID)
			if err != nil {
				return ValueChange{}, err
			}
			change.Issues = append(change.Issues, valuation.Issue{Code: "unvalued_flow", FlowID: &flowID, Symbol: flows[i].CanonicalAssetSymbol, Detail: decimal.Compact(flows[i].Quantity) + " at " + flows[i].EffectiveAt.UTC().Format(time.RFC3339)})
			continue
		}
		value, ok := decimal.Parse(flows[i].ValueUsd.String)
		if !ok {
			continue
		}
		if flows[i].Direction == valuation.DirectionContribution {
			contributions = decimal.Add(contributions, value)
		} else {
			withdrawals = decimal.Add(withdrawals, value)
		}
	}
	boundaryContributions, boundaryWithdrawals := decimal.Zero(), decimal.Zero()
	points, err := s.comparisonPointsBetween(ctx, p.ID, startPoint.EvaluatedAt.Add(time.Microsecond), endPoint.EvaluatedAt)
	if err != nil {
		return ValueChange{}, err
	}
	for i := range points {
		for _, issue := range points[i].Issues {
			if issue.Code == "asset_mapping_changed" || issue.Code == "calculation_version_changed" {
				change.Completeness = valuation.CompletenessIncomplete
				change.Issues = append(change.Issues, issue)
			}
		}
		for _, adjustment := range points[i].ManualBalanceChanges {
			change.Completeness = valuation.CompletenessIncomplete
			issue := adjustment.Issue()
			change.Issues = append(change.Issues, issue)
		}
		for j := range points[i].BoundaryEvents {
			event := &points[i].BoundaryEvents[j]
			change.BoundaryEvents = append(change.BoundaryEvents, *event)
			if !event.ValuationComplete {
				change.Completeness = valuation.CompletenessIncomplete
				change.FlowsCompleteness = valuation.CompletenessIncomplete
				change.Issues = append(change.Issues, valuation.Issue{Code: "boundary_incomplete", AccountID: &event.AccountID, AccountName: event.AccountName, Detail: "The boundary event valuation is incomplete."})
			}
			if event.ValueUSD == nil {
				change.Completeness = valuation.CompletenessIncomplete
				change.FlowsCompleteness = valuation.CompletenessIncomplete
				change.Issues = append(change.Issues, valuation.Issue{Code: "unvalued_boundary_event", AccountID: &event.AccountID, AccountName: event.AccountName, Detail: "The boundary event value is unknown."})
				continue
			}
			if event.Direction == valuation.DirectionContribution {
				boundaryContributions = decimal.Add(boundaryContributions, decimal.ParseOrZero(*event.ValueUSD))
			} else {
				boundaryWithdrawals = decimal.Add(boundaryWithdrawals, decimal.ParseOrZero(*event.ValueUSD))
			}
		}
	}
	change.ContributionsUSD = decimal.FormatPtr(contributions)
	change.WithdrawalsUSD = decimal.FormatPtr(withdrawals)
	change.BoundaryContributionsUSD = decimal.FormatPtr(boundaryContributions)
	change.BoundaryWithdrawalsUSD = decimal.FormatPtr(boundaryWithdrawals)
	amount := decimal.Sub(decimal.ParseOrZero(endPoint.TotalUSD), decimal.ParseOrZero(startPoint.TotalUSD))
	amount = decimal.Sub(amount, decimal.Add(contributions, boundaryContributions))
	amount = decimal.Add(amount, decimal.Add(withdrawals, boundaryWithdrawals))
	change.AmountUSD = decimal.FormatPtr(amount)
	return change, nil
}

func (s *Service) exclusionChangeIssues(ctx context.Context, portfolioID, startCycleID, endCycleID uuid.UUID) ([]valuation.Issue, error) {
	startIgnored, err := assetexclusion.ActiveForCycle(ctx, s.db, portfolioID, startCycleID)
	if err != nil {
		return nil, err
	}
	endIgnored, err := assetexclusion.ActiveForCycle(ctx, s.db, portfolioID, endCycleID)
	if err != nil {
		return nil, err
	}
	rows, err := mysql.Queries(ctx, s.db).ListCalculatedAssetsAtCycles(ctx, sqlcgen.ListCalculatedAssetsAtCyclesParams{PortfolioID: portfolioID[:], StartCycleID: startCycleID[:], EndCycleID: endCycleID[:]})
	if err != nil {
		return nil, err
	}
	issues := []valuation.Issue{}
	for _, row := range rows {
		assetID, err := uuid.FromBytes(row.ID)
		if err != nil {
			return nil, err
		}
		if startIgnored[assetID] != endIgnored[assetID] {
			issues = append(issues, valuation.Issue{Code: "asset_exclusion_changed", Symbol: row.Symbol, Detail: row.Platform + " " + row.Symbol})
		}
	}
	return issues, nil
}

func (s *Service) pointAtOrBefore(ctx context.Context, portfolioID uuid.UUID, at time.Time, inclusive bool) (*valuation.Point, error) {
	var (
		row sqlcgen.ValuationPoint
		err error
	)
	if inclusive {
		row, err = mysql.Queries(ctx, s.db).GetValuationPointAtOrBefore(ctx, sqlcgen.GetValuationPointAtOrBeforeParams{PortfolioID: portfolioID[:], At: sql.NullTime{Time: at.UTC(), Valid: true}})
	} else {
		row, err = mysql.Queries(ctx, s.db).GetHistoryValuationPointBefore(ctx, sqlcgen.GetHistoryValuationPointBeforeParams{PortfolioID: portfolioID[:], At: sql.NullTime{Time: at.UTC(), Valid: true}, CycleID: make([]byte, 16)})
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no point at or before the instant is a valid, checked outcome.
	}
	if err != nil {
		return nil, err
	}
	point, err := s.valuation.PointWithEvents(ctx, &row)
	if err != nil {
		return nil, err
	}
	return &point, nil
}

// Only sparse evidence-bearing points need hydration for a comparison. Ordinary
// unchanged points contribute neither flows nor completeness markers.
func (s *Service) comparisonPointsBetween(ctx context.Context, portfolioID uuid.UUID, from, to time.Time) ([]valuation.Point, error) {
	headers, err := mysql.Queries(ctx, s.db).ListComparisonPointHeadersBetween(ctx, sqlcgen.ListComparisonPointHeadersBetweenParams{PortfolioID: portfolioID[:], FromAt: from.UTC(), ToAt: to.UTC()})
	if err != nil {
		return nil, err
	}
	ids := make([][]byte, 0, len(headers))
	for _, header := range headers {
		ids = append(ids, header.CycleID)
	}
	rows, err := s.pointRowsByIDs(ctx, portfolioID, ids)
	if err != nil {
		return nil, err
	}
	return s.pointsFromRows(ctx, rows)
}

func observedDays(headers []sqlcgen.ListValuationPointHeadersBetweenRow, loc *time.Location) map[string]bool {
	seen := map[string]bool{}
	for _, header := range headers {
		seen[header.EvaluatedAt.In(loc).Format("2006-01-02")] = true
	}
	return seen
}

func daysWithoutPoints(seen map[string]bool, start, end time.Time, loc *time.Location) []time.Time {
	var missing []time.Time
	now := time.Now().In(loc)
	for day := periodStart(PeriodDay, start.In(loc)); day.Before(end); day = day.AddDate(0, 0, 1) {
		if day.After(now) {
			break
		}
		if !seen[day.Format("2006-01-02")] {
			missing = append(missing, day)
		}
	}
	return missing
}

func joinDays(days []time.Time) string {
	var out strings.Builder
	for i, d := range days {
		if i == 5 {
			fmt.Fprintf(&out, ", +%d more", len(days)-5)
			break
		}
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(d.Format("2006-01-02"))
	}
	return out.String()
}
