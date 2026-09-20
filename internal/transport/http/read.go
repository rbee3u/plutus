package httpserver

import (
	"context"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/rbee3u/plutus/internal/projection"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) GetOverview(ctx context.Context, request api.GetOverviewRequestObject) (api.GetOverviewResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.GetOverview401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.GetOverview404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Projection == nil {
		return nil, ErrNotImplemented
	}
	item, err := a.Projection.Overview(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	return api.GetOverview200JSONResponse(toAPIOverview(&item)), nil
}

func (a *API) ListBalances(ctx context.Context, request api.ListBalancesRequestObject) (api.ListBalancesResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListBalances401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListBalances404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Projection == nil {
		return nil, ErrNotImplemented
	}
	filter := projection.ListFilter{Offset: intValue(request.Params.Offset), Limit: intValue(request.Params.Limit)}
	if request.Params.GroupBy != nil {
		filter.GroupBy = string(*request.Params.GroupBy)
	}
	filter.IdentityID = uuidPtr(request.Params.IdentityId)
	filter.AccountID = uuidPtr(request.Params.AccountId)
	filter.AssetID = uuidPtr(request.Params.AssetId)
	filter.ListingID = uuidPtr(request.Params.ListingId)
	page, err := a.Projection.ListBalances(ctx, request.PortfolioId, &filter)
	if err != nil {
		return nil, err
	}
	items := make([]api.BalanceRow, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIBalanceRow(&page.Items[i]))
	}
	return api.ListBalances200JSONResponse{
		Items: items, Groups: toAPIGroups(page.Groups), Page: toOffsetPage(page.Offset, page.Limit, page.Total),
		Context: toAPIContext(&page.Context), Issues: toAPIIssues(page.Issues),
	}, nil
}

func (a *API) ListPositions(ctx context.Context, request api.ListPositionsRequestObject) (api.ListPositionsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListPositions401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListPositions404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Projection == nil {
		return nil, ErrNotImplemented
	}
	filter := projection.ListFilter{Offset: intValue(request.Params.Offset), Limit: intValue(request.Params.Limit)}
	if request.Params.GroupBy != nil {
		filter.GroupBy = string(*request.Params.GroupBy)
	}
	filter.IdentityID = uuidPtr(request.Params.IdentityId)
	filter.AccountID = uuidPtr(request.Params.AccountId)
	filter.InstrumentID = uuidPtr(request.Params.InstrumentId)
	page, err := a.Projection.ListPositions(ctx, request.PortfolioId, &filter)
	if err != nil {
		return nil, err
	}
	items := make([]api.PositionRow, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIPositionRow(&page.Items[i]))
	}
	return api.ListPositions200JSONResponse{
		Items: items, Groups: toAPIGroups(page.Groups), Page: toOffsetPage(page.Offset, page.Limit, page.Total),
		Context: toAPIContext(&page.Context), Issues: toAPIIssues(page.Issues),
	}, nil
}

func (a *API) ListExposures(ctx context.Context, request api.ListExposuresRequestObject) (api.ListExposuresResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListExposures401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListExposures404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Projection == nil {
		return nil, ErrNotImplemented
	}
	if request.Params.GroupBy != nil && !request.Params.GroupBy.Valid() {
		return nil, projection.ErrInvalidGroup
	}
	filter := projection.ExposureFilter{Offset: intValue(request.Params.Offset), Limit: intValue(request.Params.Limit)}
	if request.Params.GroupBy != nil {
		filter.GroupBy = string(*request.Params.GroupBy)
	}
	filter.IdentityID = uuidPtr(request.Params.IdentityId)
	filter.AccountID = uuidPtr(request.Params.AccountId)
	filter.AssetID = uuidPtr(request.Params.AssetId)
	page, err := a.Projection.ListExposures(ctx, request.PortfolioId, &filter)
	if err != nil {
		return nil, err
	}
	items := make([]api.ExposureRow, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIExposureRow(&page.Items[i]))
	}
	return api.ListExposures200JSONResponse{
		Items: items, Groups: toAPIGroups(page.Groups), Page: toOffsetPage(page.Offset, page.Limit, page.Total),
		Context: toAPIContext(&page.Context), Issues: toAPIIssues(page.Issues),
	}, nil
}

func (a *API) ListValuations(ctx context.Context, request api.ListValuationsRequestObject) (api.ListValuationsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListValuations401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListValuations404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Projection == nil {
		return nil, ErrNotImplemented
	}
	if request.Params.Granularity != nil && !request.Params.Granularity.Valid() {
		return nil, projection.ErrInvalidQuery
	}
	page, err := a.Projection.ListValuations(ctx, request.PortfolioId, seriesFilter(request.Params.From, request.Params.To, request.Params.Granularity, request.Params.MaxPoints, nil, request.Params.Limit, request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	return api.ListValuations200JSONResponse(toAPIValuationList(&page)), nil
}

func (a *API) ListValueChanges(ctx context.Context, request api.ListValueChangesRequestObject) (api.ListValueChangesResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListValueChanges401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListValueChanges404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Projection == nil {
		return nil, ErrNotImplemented
	}
	if request.Params.Granularity != nil && !request.Params.Granularity.Valid() {
		return nil, projection.ErrInvalidQuery
	}
	if request.Params.PeriodKind != nil && !request.Params.PeriodKind.Valid() {
		return nil, projection.ErrInvalidQuery
	}
	page, err := a.Projection.ListValueChanges(ctx, request.PortfolioId, seriesFilter(request.Params.From, request.Params.To, request.Params.Granularity, request.Params.MaxPoints, request.Params.PeriodKind, request.Params.Limit, request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	return api.ListValueChanges200JSONResponse(toAPIValueChangeList(&page)), nil
}

func (a *API) ListPeriodSummaries(ctx context.Context, request api.ListPeriodSummariesRequestObject) (api.ListPeriodSummariesResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListPeriodSummaries401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListPeriodSummaries404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Projection == nil {
		return nil, ErrNotImplemented
	}
	if request.Params.PeriodKind != nil && !request.Params.PeriodKind.Valid() {
		return nil, projection.ErrInvalidQuery
	}
	in := projection.SeriesFilter{From: request.Params.From, To: request.Params.To, Limit: intValue(request.Params.Limit), Cursor: stringValue(request.Params.Cursor)}
	if request.Params.PeriodKind != nil {
		in.PeriodKind = string(*request.Params.PeriodKind)
	}
	page, err := a.Projection.ListPeriodSummaries(ctx, request.PortfolioId, &in)
	if err != nil {
		return nil, err
	}
	items := make([]api.PeriodSummary, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIPeriodSummary(&page.Items[i]))
	}
	return api.ListPeriodSummaries200JSONResponse{
		Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore),
		Context: toAPIContext(&page.Context), Issues: toAPIIssues(page.Issues),
	}, nil
}

func toAPIOverview(item *projection.Overview) api.Overview {
	return api.Overview{
		PortfolioId:        item.PortfolioID,
		Context:            toAPIContext(&item.Context),
		Valuation:          toAPIAmount(&item.Valuation),
		ContributedCapital: toAPIAmount(&item.ContributedCapital),
		ValueChange:        toAPIAmount(&item.ValueChange),
		ValuationSeries:    toAPIValuationSeries(item.ValuationSeries),
		ValueChanges:       toAPIValueChanges(item.ValueChanges),
		Exposures:          toAPIExposures(item.Exposures),
		Issues:             toAPIIssues(item.Issues),
	}
}

func toAPIContext(item *projection.Context) api.ProjectionContext {
	out := api.ProjectionContext{
		Timezone:     item.Timezone,
		DisplayAsset: api.DisplayAsset{Id: item.DisplayAssetID, Code: item.DisplayCode},
		CutoffAt:     item.CutoffAt,
		Completeness: api.Completeness(item.Completeness),
		Coverage:     api.Coverage(item.Coverage),
		Freshness:    api.Freshness(item.Freshness),
	}
	if item.GenerationID != nil {
		id := *item.GenerationID
		out.GenerationId = &id
	}
	if item.RebuildPending {
		pending := true
		out.RebuildPending = &pending
	}
	return out
}

func toAPIAmount(item *projection.Amount) api.AmountStatus {
	out := api.AmountStatus{
		Amount:       item.Amount,
		Completeness: api.Completeness(item.Completeness),
		Coverage:     api.Coverage(item.Coverage),
		Freshness:    api.Freshness(item.Freshness),
		EvaluatedAt:  item.EvaluatedAt,
	}
	if item.State != "" {
		out.State = &item.State
	}
	if item.PeriodKind != "" {
		out.PeriodKind = &item.PeriodKind
	}
	if item.PeriodStartDate != nil {
		d := toAPIDate(*item.PeriodStartDate)
		out.PeriodStartDate = &d
	}
	if item.PeriodEndDate != nil {
		d := toAPIDate(*item.PeriodEndDate)
		out.PeriodEndDate = &d
	}
	return out
}

func toAPIValuationSeries(items []projection.ValuationPoint) []api.ValuationPoint {
	out := make([]api.ValuationPoint, 0, len(items))
	for i := range items {
		out = append(out, api.ValuationPoint{
			EvaluatedAt: items[i].EvaluatedAt, Amount: items[i].Amount, Completeness: api.Completeness(items[i].Completeness),
		})
	}
	return out
}

func toAPIValueChanges(items []projection.ValueChangePoint) []api.ValueChangePoint {
	out := make([]api.ValueChangePoint, 0, len(items))
	for i := range items {
		out = append(out, api.ValueChangePoint{
			PeriodKind:      api.ValueChangePointPeriodKind(items[i].PeriodKind),
			PeriodStartDate: toAPIDate(items[i].PeriodStart),
			PeriodEndDate:   toAPIDate(items[i].PeriodEnd),
			Amount:          items[i].Amount,
			State:           api.ValueChangePointState(items[i].State),
		})
	}
	return out
}

func toAPIExposures(items []projection.ExposurePoint) []api.ExposurePoint {
	out := make([]api.ExposurePoint, 0, len(items))
	for i := range items {
		code := items[i].AssetCode
		out = append(out, api.ExposurePoint{
			AssetId: items[i].AssetID, AssetCode: strPtr(code),
			LongQuantity: items[i].Long, ShortQuantity: items[i].Short, NetQuantity: items[i].Net,
			LongValue: items[i].LongValue, ShortValue: items[i].ShortValue, NetValue: items[i].NetValue,
			Completeness: api.Completeness(items[i].Completeness), Freshness: api.Freshness(items[i].Freshness),
		})
	}
	return out
}

func toAPIIssues(items []projection.Issue) []api.ProjectionIssue {
	out := make([]api.ProjectionIssue, 0, len(items))
	for i := range items {
		item := api.ProjectionIssue{
			Code: items[i].Code, Severity: api.ProjectionIssueSeverity(items[i].Severity),
			Summary: items[i].Summary, ProjectionKind: items[i].Kind,
		}
		if items[i].Key != "" {
			item.ProjectionKey = &items[i].Key
		}
		item.Id = items[i].ID
		item.IdentityId = items[i].IdentityID
		item.AccountId = items[i].AccountID
		item.ObservationId = items[i].ObservationID
		first, last := items[i].First, items[i].Last
		if !first.IsZero() {
			item.FirstEffectiveAt = &first
		}
		if !last.IsZero() {
			item.LastEffectiveAt = &last
		}
		out = append(out, item)
	}
	return out
}

func toAPIBalanceRow(item *projection.BalanceRow) api.BalanceRow {
	out := api.BalanceRow{
		IdentityId: item.IdentityID, AccountId: item.AccountID, AssetListingId: item.ListingID,
		TotalQuantity: item.Total, AvailableQuantity: item.Available, LockedQuantity: item.Locked,
		SourceObservedAt: item.ObservedAt, Freshness: api.Freshness(item.Freshness), Completeness: api.Completeness(item.Completeness),
		IdentityName: strPtr(item.IdentityName), AccountName: strPtr(item.AccountName), ListingSymbol: strPtr(item.ListingSymbol),
	}
	if item.PlatformID != uuid.Nil {
		id := item.PlatformID
		out.PlatformId = &id
		out.PlatformCode = strPtr(item.PlatformCode)
	}
	out.AssetId = item.AssetID
	if item.AssetCode != "" {
		out.AssetCode = &item.AssetCode
	}
	return out
}

func toAPIPositionRow(item *projection.PositionRow) api.PositionRow {
	out := api.PositionRow{
		IdentityId: item.IdentityID, AccountId: item.AccountID, InstrumentId: item.InstrumentID,
		PlatformPositionRef: item.Ref, Side: api.PositionRowSide(item.Side), Quantity: item.Quantity,
		UnderlyingLongQuantity: item.Long, UnderlyingShortQuantity: item.Short, SelectedMarkPrice: item.Mark,
		SourceObservedAt: item.ObservedAt, Freshness: api.Freshness(item.Freshness), Completeness: api.Completeness(item.Completeness),
		IdentityName: strPtr(item.IdentityName), AccountName: strPtr(item.AccountName), InstrumentSymbol: strPtr(item.Symbol),
	}
	if item.PlatformID != uuid.Nil {
		id := item.PlatformID
		out.PlatformId = &id
		out.PlatformCode = strPtr(item.PlatformCode)
	}
	return out
}

func toAPIGroups(items []projection.Group) []api.ProjectionGroup {
	out := make([]api.ProjectionGroup, 0, len(items))
	for i := range items {
		out = append(out, api.ProjectionGroup{
			Grouping: items[i].Grouping, Key: items[i].Key, Label: items[i].Label,
			Completeness: api.Completeness(items[i].Completeness), Freshness: api.Freshness(items[i].Freshness),
			Totals: api.GroupTotals{
				TotalQuantity: items[i].Totals.Total, AvailableQuantity: items[i].Totals.Available, LockedQuantity: items[i].Totals.Locked,
				Quantity: items[i].Totals.Quantity, UnderlyingLongQuantity: items[i].Totals.Long, UnderlyingShortQuantity: items[i].Totals.Short,
				LongQuantity: items[i].Totals.LongQuantity, ShortQuantity: items[i].Totals.ShortQuantity, NetQuantity: items[i].Totals.NetQuantity,
				LongValue: items[i].Totals.LongValue, ShortValue: items[i].Totals.ShortValue, NetValue: items[i].Totals.NetValue,
			},
		})
	}
	return out
}

func toAPIDate(value time.Time) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)}
}

func strPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func uuidPtr(value *api.UUID) *uuid.UUID {
	return value
}

func seriesFilter(from, to *time.Time, grain *api.Granularity, maxPoints *int, period *api.PeriodKind, limit *int, cursor *string) *projection.SeriesFilter {
	in := &projection.SeriesFilter{From: from, To: to, Limit: intValue(limit), Cursor: stringValue(cursor), MaxPoints: intValue(maxPoints)}
	if grain != nil {
		in.Granularity = string(*grain)
	}
	if period != nil {
		in.PeriodKind = string(*period)
	}
	return in
}

func toAPIExposureRow(item *projection.ExposureRow) api.ExposureRow {
	out := api.ExposureRow{
		AssetId: item.AssetID, AssetCode: strPtr(item.AssetCode),
		LongQuantity: item.Long, ShortQuantity: item.Short, NetQuantity: item.Net,
		LongValue: item.LongValue, ShortValue: item.ShortValue, NetValue: item.NetValue,
		Completeness: api.Completeness(item.Completeness), Freshness: api.Freshness(item.Freshness),
		EvaluatedAt: item.EvaluatedAt, IdentityName: strPtr(item.IdentityName), AccountName: strPtr(item.AccountName),
		PlatformCode: strPtr(item.PlatformCode),
	}
	out.IdentityId = item.IdentityID
	out.AccountId = item.AccountID
	out.PlatformId = item.PlatformID
	return out
}

func toAPIValuationList(page *projection.ValuationPage) api.ValuationList {
	out := api.ValuationList{
		Items: toAPIValuationSeries(page.Items), Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore),
		Context: toAPIContext(&page.Context), Issues: toAPIIssues(page.Issues),
	}
	if page.Granularity != "" {
		grain := api.TimeGranularity(page.Granularity)
		out.Granularity = &grain
	}
	if intervals := toAPIMissing(page.MissingIntervals); intervals != nil {
		out.MissingIntervals = &intervals
	}
	return out
}

func toAPIValueChangeList(page *projection.ValueChangePage) api.ValueChangeList {
	items := make([]api.ValueChangeRow, 0, len(page.Items))
	for i := range page.Items {
		item := api.ValueChangeRow{
			PeriodKind:      api.CalendarPeriodKind(page.Items[i].PeriodKind),
			PeriodStartDate: toAPIDate(page.Items[i].PeriodStart), PeriodEndDate: toAPIDate(page.Items[i].PeriodEnd),
			Amount: page.Items[i].Amount, State: api.ValueChangeRowState(page.Items[i].State),
			ContributionAmount: page.Items[i].ContributionAmount, WithdrawalAmount: page.Items[i].WithdrawalAmount,
		}
		if page.Items[i].CoverageFingerprint != "" {
			fp := page.Items[i].CoverageFingerprint
			item.CoverageFingerprint = &fp
		}
		items = append(items, item)
	}
	out := api.ValueChangeList{
		Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore),
		Context: toAPIContext(&page.Context), Issues: toAPIIssues(page.Issues),
	}
	if page.Granularity != "" {
		grain := api.TimeGranularity(page.Granularity)
		out.Granularity = &grain
	}
	if intervals := toAPIMissing(page.MissingIntervals); intervals != nil {
		out.MissingIntervals = &intervals
	}
	return out
}

func toAPIPeriodSummary(item *projection.PeriodSummaryRow) api.PeriodSummary {
	asOf := item.AsOfAt
	return api.PeriodSummary{
		PeriodKind:      api.CalendarPeriodKind(item.PeriodKind),
		PeriodStartDate: toAPIDate(item.PeriodStart), PeriodEndDate: toAPIDate(item.PeriodEnd),
		AsOfAt: &asOf, ValuationOpen: item.ValuationOpen, ValuationClose: item.ValuationClose,
		ValuationHigh: item.ValuationHigh, ValuationLow: item.ValuationLow,
		ContributedCapitalClose: item.ContributedCapitalClose, ValueChange: item.ValueChange,
		Completeness: api.Completeness(item.Completeness), Freshness: api.Freshness(item.Freshness),
	}
}

func toAPIMissing(items []projection.MissingInterval) []api.MissingInterval {
	if len(items) == 0 {
		return nil
	}
	out := make([]api.MissingInterval, 0, len(items))
	for i := range items {
		out = append(out, api.MissingInterval{Start: items[i].Start, End: items[i].End})
	}
	return out
}
