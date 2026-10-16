package httpserver

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

// timeRange applies the default window when from or to is omitted.
func timeRange(from, to *time.Time, defaultSpan time.Duration) (time.Time, time.Time) {
	end := time.Now().UTC()
	if to != nil {
		end = to.UTC()
	}
	start := end.Add(-defaultSpan)
	if from != nil {
		start = from.UTC()
	}
	return start, end
}

// periodWindow is timeRange for period lists: an omitted from starts no earlier
// than the portfolio's first valuation point, so the list does not open with
// periods from before there was anything to value. Without any point the
// window is empty.
func (a *API) periodWindow(ctx context.Context, portfolioID uuid.UUID, from, to *time.Time, kind string) (time.Time, time.Time, bool, error) {
	start, end := timeRange(from, to, defaultSpanFor(kind))
	if from != nil {
		return start, end, true, nil
	}
	first, err := a.Valuation.First(ctx, portfolioID)
	if errors.Is(err, valuation.ErrNoPoint) {
		return start, end, false, nil
	}
	if err != nil {
		return start, end, false, err
	}
	if first.EvaluatedAt.After(start) {
		start = first.EvaluatedAt.UTC()
	}
	return start, end, start.Before(end), nil
}

func periodKind(v *api.PeriodKind) string {
	if v == nil {
		return financial.PeriodDay
	}
	return string(*v)
}

func (a *API) ListValuationPoints(ctx context.Context, request api.ListValuationPointsRequestObject) (api.ListValuationPointsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil || a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	from, to := timeRange(request.Params.From, request.Params.To, 7*24*time.Hour)
	granularity := api.TimeGranularityRaw
	if request.Params.Granularity != nil {
		granularity = *request.Params.Granularity
	}
	points, err := a.Financial.Points(ctx, sc.portfolioContext(), from, to, string(granularity))
	if err != nil {
		return nil, err
	}
	c := financial.Context{PortfolioID: sc.portfolio.ID, Timezone: sc.portfolio.Timezone}
	return api.ListValuationPoints200JSONResponse{Context: toAPIContext(&c), Items: toAPIPoints(points), Granularity: granularity, From: from, To: to}, nil
}

func (a *API) GetValuationPoint(ctx context.Context, request api.GetValuationPointRequestObject) (api.GetValuationPointResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil || a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	detail, err := a.Financial.Detail(ctx, sc.portfolioContext(), request.CycleId)
	if err != nil {
		return nil, err
	}
	return api.GetValuationPoint200JSONResponse{
		Point:          toAPIPoint(&detail.Point),
		Accounts:       toAPIAccountValuations(detail.Accounts),
		Issues:         toAPIIssues(detail.Point.Issues),
		BoundaryEvents: toAPIBoundaryEvents(detail.Point.BoundaryEvents),
	}, nil
}

func (a *API) ListPeriodCloses(ctx context.Context, request api.ListPeriodClosesRequestObject) (api.ListPeriodClosesResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil || a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	kind := periodKind(request.Params.PeriodKind)
	from, to, ok, err := a.periodWindow(ctx, sc.portfolio.ID, request.Params.From, request.Params.To, kind)
	if err != nil {
		return nil, err
	}
	var closes []financial.PeriodClose
	if ok {
		if closes, err = a.Financial.PeriodCloses(ctx, sc.portfolioContext(), kind, from, to); err != nil {
			return nil, err
		}
	}
	items := make([]api.PeriodClose, 0, len(closes))
	for i := range closes {
		items = append(items, toAPIPeriodClose(&closes[i]))
	}
	c := financial.Context{PortfolioID: sc.portfolio.ID, Timezone: sc.portfolio.Timezone}
	return api.ListPeriodCloses200JSONResponse{Context: toAPIContext(&c), PeriodKind: api.PeriodKind(kind), Items: items}, nil
}

func (a *API) ListValueChanges(ctx context.Context, request api.ListValueChangesRequestObject) (api.ListValueChangesResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil || a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	kind := periodKind(request.Params.PeriodKind)
	from, to, ok, err := a.periodWindow(ctx, sc.portfolio.ID, request.Params.From, request.Params.To, kind)
	if err != nil {
		return nil, err
	}
	var changes []financial.ValueChange
	if ok {
		if changes, err = a.Financial.ValueChanges(ctx, sc.portfolioContext(), kind, from, to); err != nil {
			return nil, err
		}
	}
	items := make([]api.ValueChange, 0, len(changes))
	for i := range changes {
		items = append(items, toAPIValueChange(&changes[i]))
	}
	c := financial.Context{PortfolioID: sc.portfolio.ID, Timezone: sc.portfolio.Timezone}
	return api.ListValueChanges200JSONResponse{Context: toAPIContext(&c), PeriodKind: api.PeriodKind(kind), Items: items}, nil
}

func (a *API) GetCustomValueChange(ctx context.Context, request api.GetCustomValueChangeRequestObject) (api.GetCustomValueChangeResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil || a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	change, err := a.Financial.CustomValueChange(ctx, sc.portfolioContext(), request.Params.From.UTC(), request.Params.To.UTC())
	if err != nil {
		return nil, err
	}
	return api.GetCustomValueChange200JSONResponse(toAPIValueChange(&change)), nil
}

func (a *API) ListBoundaryEvents(ctx context.Context, request api.ListBoundaryEventsRequestObject) (api.ListBoundaryEventsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil || a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	from, to := timeRange(request.Params.From, request.Params.To, 90*24*time.Hour)
	events, err := a.Financial.BoundaryEvents(ctx, sc.portfolioContext(), from, to)
	if err != nil {
		return nil, err
	}
	return api.ListBoundaryEvents200JSONResponse{Items: toAPIBoundaryEvents(events)}, nil
}

// defaultSpanFor picks a window that shows a useful number of periods.
func defaultSpanFor(kind string) time.Duration {
	switch kind {
	case financial.PeriodWeek:
		return 26 * 7 * 24 * time.Hour
	case financial.PeriodMonth:
		return 366 * 24 * time.Hour
	case financial.PeriodQuarter:
		return 3 * 366 * 24 * time.Hour
	case financial.PeriodYear:
		return 5 * 366 * 24 * time.Hour
	default:
		return 30 * 24 * time.Hour
	}
}

func (a *API) GetHistoryWindow(ctx context.Context, request api.GetHistoryWindowRequestObject) (api.GetHistoryWindowResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	preset, fromLocal, toLocal := "", "", ""
	if request.Params.Preset != nil {
		preset = string(*request.Params.Preset)
	}
	if request.Params.FromLocal != nil {
		fromLocal = *request.Params.FromLocal
	}
	if request.Params.ToLocal != nil {
		toLocal = *request.Params.ToLocal
	}
	if (request.Params.FromLocal != nil || request.Params.ToLocal != nil) && (request.Params.Preset != nil || fromLocal == "" || toLocal == "") {
		if request.Params.Preset == nil {
			return nil, validationError("Enter both a start and an end date and time.")
		}
		return nil, validationError("Supply both local endpoints without a preset.")
	}
	from, to, err := financial.ResolveHistoryWindow(sc.portfolio.Timezone, preset, fromLocal, toLocal, time.Now())
	if err != nil {
		return nil, err
	}
	c := financial.Context{PortfolioID: sc.portfolio.ID, Timezone: sc.portfolio.Timezone}
	return api.GetHistoryWindow200JSONResponse{Context: toAPIContext(&c), From: from, To: to}, nil
}

func (a *API) ListValuationPointPage(ctx context.Context, request api.ListValuationPointPageRequestObject) (api.ListValuationPointPageResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil || a.Valuation == nil {
		return nil, ErrNotImplemented
	}
	from, to := timeRange(request.Params.From, request.Params.To, 7*24*time.Hour)
	offset, limit := 0, 20
	if request.Params.Offset != nil {
		offset = *request.Params.Offset
	}
	if request.Params.Limit != nil {
		limit = int(*request.Params.Limit)
	}
	page, err := a.Financial.PointsPage(ctx, sc.portfolioContext(), from, to, offset, limit)
	if err != nil {
		return nil, err
	}
	c := financial.Context{PortfolioID: sc.portfolio.ID, Timezone: sc.portfolio.Timezone}
	return api.ListValuationPointPage200JSONResponse{Context: toAPIContext(&c), From: from, To: to, Items: toAPIPoints(page.Items), Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}

func (a *API) GetValuationSnapshot(ctx context.Context, request api.GetValuationSnapshotRequestObject) (api.GetValuationSnapshotResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	detail, err := a.Financial.Snapshot(ctx, sc.portfolioContext(), request.Params.Day)
	if err != nil {
		return nil, err
	}
	return api.GetValuationSnapshot200JSONResponse{Point: toAPIPoint(&detail.Point), Accounts: toAPIAccountValuations(detail.Accounts), Issues: toAPIIssues(detail.Point.Issues), BoundaryEvents: toAPIBoundaryEvents(detail.Point.BoundaryEvents)}, nil
}
