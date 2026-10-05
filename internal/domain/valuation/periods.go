package valuation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/domain/listingexclusion"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Period kinds.
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

// periodsBetween enumerates local calendar periods of kind that intersect [from, to].
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

// PeriodCloses returns the Period Close of every period of kind between two instants.
func (s *Service) PeriodCloses(ctx context.Context, p PortfolioContext, kind string, from, to time.Time) ([]PeriodClose, error) {
	loc := p.location()
	periods, err := periodsBetween(kind, from, to, loc)
	if err != nil {
		return nil, err
	}
	if len(periods) == 0 {
		return []PeriodClose{}, nil
	}
	points, err := s.pointsInRange(ctx, p.ID, periods[0].start, periods[len(periods)-1].end)
	if err != nil {
		return nil, err
	}
	out := make([]PeriodClose, 0, len(periods))
	for _, per := range periods {
		item := PeriodClose{PeriodKind: kind, Label: per.label, PeriodStart: per.start, PeriodEnd: per.end, Completeness: CompletenessComplete, Reasons: []string{}}
		point := lastPointBefore(points, per.end, per.start)
		if point == nil {
			item.Completeness = CompletenessIncomplete
			item.Reasons = append(item.Reasons, "no_point_in_period")
		} else {
			item.Point = point
			if point.Completeness() == CompletenessIncomplete {
				item.Completeness = CompletenessIncomplete
				item.Reasons = append(item.Reasons, "point_incomplete")
			}
			if kind != PeriodDay {
				if missing := daysWithoutPoints(points, per.start, per.end, loc); len(missing) > 0 {
					item.Completeness = CompletenessIncomplete
					item.Reasons = append(item.Reasons, "days_without_points: "+joinDays(missing))
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// ValueChanges returns the Value Change of every period of kind between two instants.
func (s *Service) ValueChanges(ctx context.Context, p PortfolioContext, kind string, from, to time.Time) ([]ValueChange, error) {
	loc := p.location()
	periods, err := periodsBetween(kind, from, to, loc)
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
}

// CustomValueChange returns one Value Change between the last points at or before two instants.
func (s *Service) CustomValueChange(ctx context.Context, p PortfolioContext, from, to time.Time) (ValueChange, error) {
	if !to.After(from) {
		return ValueChange{}, ErrInvalid
	}
	return s.valueChange(ctx, p, PeriodCustom, "custom", from, to)
}

// valueChange computes end − start − contributions + withdrawals where start is the last point
// before periodStart and end the last point before periodEnd (both exclusive boundaries).
func (s *Service) valueChange(ctx context.Context, p PortfolioContext, kind, label string, periodStart, periodEnd time.Time) (ValueChange, error) {
	change := ValueChange{PeriodKind: kind, Label: label, From: periodStart, To: periodEnd, Completeness: CompletenessComplete, Reasons: []string{}, BoundaryEvents: []BoundaryEvent{}}
	change.ContributionsUSD, change.WithdrawalsUSD, change.BoundaryContributionsUSD, change.BoundaryWithdrawalsUSD = "0", "0", "0", "0"
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
		change.Completeness = CompletenessIncomplete
		change.Reasons = append(change.Reasons, "no_start_point")
	}
	if endPoint == nil {
		change.Completeness = CompletenessIncomplete
		change.Reasons = append(change.Reasons, "no_end_point")
	}
	if startPoint == nil || endPoint == nil {
		return change, nil
	}
	if !endPoint.EvaluatedAt.After(startPoint.EvaluatedAt) {
		change.Completeness = CompletenessIncomplete
		change.Reasons = append(change.Reasons, "no_point_in_period")
		amount := "0"
		change.AmountUSD = &amount
		return change, nil
	}
	if startPoint.Completeness() == CompletenessIncomplete {
		change.Completeness = CompletenessIncomplete
		change.Reasons = append(change.Reasons, "start_point_incomplete")
	}
	if endPoint.Completeness() == CompletenessIncomplete {
		change.Completeness = CompletenessIncomplete
		change.Reasons = append(change.Reasons, "end_point_incomplete")
	}
	if kind != PeriodCustom && kind != PeriodDay {
		if points, err := s.pointsInRange(ctx, p.ID, periodStart, periodEnd); err != nil {
			return ValueChange{}, err
		} else if missing := daysWithoutPoints(points, periodStart, periodEnd, p.location()); len(missing) > 0 {
			change.Completeness = CompletenessIncomplete
			change.Reasons = append(change.Reasons, "days_without_points: "+joinDays(missing))
		}
	}
	policyReasons, err := s.exclusionChangeReasons(ctx, p.ID, startPoint.JobID, endPoint.JobID)
	if err != nil {
		return ValueChange{}, err
	}
	if len(policyReasons) > 0 {
		change.Completeness = CompletenessIncomplete
		change.Reasons = append(change.Reasons, policyReasons...)
	}
	contributions, withdrawals := decimal.Zero(), decimal.Zero()
	flows, err := s.q.ListCapitalFlows(ctx, sqlcgen.ListCapitalFlowsParams{PortfolioID: p.ID[:], FromAt: startPoint.EvaluatedAt.Add(time.Microsecond), ToAt: endPoint.EvaluatedAt.Add(time.Microsecond)})
	if err != nil {
		return ValueChange{}, err
	}
	for i := range flows {
		if !flows[i].ValueUsd.Valid {
			change.Completeness = CompletenessIncomplete
			change.Reasons = append(change.Reasons, "unvalued_flow: "+flows[i].AssetSymbol+" "+decimal.Compact(flows[i].Quantity)+" at "+flows[i].EffectiveAt.UTC().Format(time.RFC3339))
			continue
		}
		value, ok := decimal.Parse(flows[i].ValueUsd.String)
		if !ok {
			continue
		}
		if flows[i].Direction == DirectionContribution {
			contributions = decimal.Add(contributions, value)
		} else {
			withdrawals = decimal.Add(withdrawals, value)
		}
	}
	boundaryContributions, boundaryWithdrawals := decimal.Zero(), decimal.Zero()
	points, err := s.pointsInRange(ctx, p.ID, startPoint.EvaluatedAt.Add(time.Microsecond), endPoint.EvaluatedAt)
	if err != nil {
		return ValueChange{}, err
	}
	for i := range points {
		for _, issue := range points[i].Issues {
			if issue.Code == "account_scope_changed" || issue.Code == "manual_balance_changed" {
				change.Completeness = CompletenessIncomplete
				change.Reasons = append(change.Reasons, issue.Code+": "+issue.Detail)
			}
		}
		for j := range points[i].BoundaryEvents {
			event := &points[i].BoundaryEvents[j]
			change.BoundaryEvents = append(change.BoundaryEvents, *event)
			if event.valueRat == nil {
				change.Completeness = CompletenessIncomplete
				change.Reasons = append(change.Reasons, "unvalued_boundary_event: "+event.AccountName)
				continue
			}
			if event.Direction == DirectionContribution {
				boundaryContributions = decimal.Add(boundaryContributions, event.valueRat)
			} else {
				boundaryWithdrawals = decimal.Add(boundaryWithdrawals, event.valueRat)
			}
		}
	}
	change.ContributionsUSD = decimal.Format(contributions)
	change.WithdrawalsUSD = decimal.Format(withdrawals)
	change.BoundaryContributionsUSD = decimal.Format(boundaryContributions)
	change.BoundaryWithdrawalsUSD = decimal.Format(boundaryWithdrawals)
	amount := decimal.Sub(decimal.MustParse(endPoint.TotalUSD), decimal.MustParse(startPoint.TotalUSD))
	amount = decimal.Sub(amount, decimal.Add(contributions, boundaryContributions))
	amount = decimal.Add(amount, decimal.Add(withdrawals, boundaryWithdrawals))
	change.AmountUSD = decimal.FormatPtr(amount)
	return change, nil
}

func (s *Service) exclusionChangeReasons(ctx context.Context, portfolioID, startJobID, endJobID uuid.UUID) ([]string, error) {
	startIgnored, err := listingexclusion.ActiveForJob(ctx, s.db, portfolioID, startJobID)
	if err != nil {
		return nil, err
	}
	endIgnored, err := listingexclusion.ActiveForJob(ctx, s.db, portfolioID, endJobID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT l.id, l.platform, l.symbol
		FROM account_reports r
		JOIN balances b ON b.report_id = r.source_report_id
		JOIN asset_listings l ON l.id = b.listing_id
		WHERE r.portfolio_id = ? AND r.job_id IN (?, ?)
		  AND r.method = 'calculated' AND b.equity <> 0
		ORDER BY l.platform, l.symbol`, portfolioID[:], startJobID[:], endJobID[:])
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	reasons := []string{}
	for rows.Next() {
		var raw []byte
		var platform, symbol string
		if err := rows.Scan(&raw, &platform, &symbol); err != nil {
			return nil, err
		}
		listingID, err := uuid.FromBytes(raw)
		if err != nil {
			return nil, err
		}
		if startIgnored[listingID] != endIgnored[listingID] {
			reasons = append(reasons, "listing_exclusion_changed: "+platform+" "+symbol)
		}
	}
	return reasons, rows.Err()
}

func (s *Service) pointAtOrBefore(ctx context.Context, portfolioID uuid.UUID, at time.Time, inclusive bool) (*Point, error) {
	var (
		row sqlcgen.ValuationPoint
		err error
	)
	if inclusive {
		row, err = s.q.GetValuationPointAtOrBefore(ctx, sqlcgen.GetValuationPointAtOrBeforeParams{PortfolioID: portfolioID[:], At: at.UTC()})
	} else {
		row, err = s.q.GetValuationPointBefore(ctx, sqlcgen.GetValuationPointBeforeParams{PortfolioID: portfolioID[:], At: at.UTC()})
	}
	if err != nil {
		if isNoRows(err) {
			return nil, nil //nolint:nilnil // no point at or before the instant is a valid, checked outcome.
		}
		return nil, err
	}
	point, err := pointFromRow(&row)
	if err != nil {
		return nil, err
	}
	return &point, nil
}

func (s *Service) pointsInRange(ctx context.Context, portfolioID uuid.UUID, from, to time.Time) ([]Point, error) {
	rows, err := s.q.ListValuationPointsBetween(ctx, sqlcgen.ListValuationPointsBetweenParams{PortfolioID: portfolioID[:], FromAt: from.UTC(), ToAt: to.UTC()})
	if err != nil {
		return nil, err
	}
	return pointsFromRows(rows)
}

func lastPointBefore(points []Point, before, notBefore time.Time) *Point {
	var found *Point
	for i := range points {
		at := points[i].EvaluatedAt
		if at.Before(before) && !at.Before(notBefore) {
			found = &points[i]
		}
	}
	return found
}

func daysWithoutPoints(points []Point, start, end time.Time, loc *time.Location) []time.Time {
	seen := map[string]bool{}
	for i := range points {
		local := points[i].EvaluatedAt.In(loc)
		seen[local.Format("2006-01-02")] = true
	}
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
