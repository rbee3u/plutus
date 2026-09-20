package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/observation"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListAccounts(ctx context.Context, request api.ListAccountsRequestObject) (api.ListAccountsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListAccounts401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListAccounts404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Observations == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Observations.ListAccounts(ctx, request.PortfolioId, intValue(request.Params.Offset), intValue(request.Params.Limit))
	if err != nil {
		return nil, err
	}
	items := make([]api.Account, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIAccount(&page.Items[i]))
	}
	return api.ListAccounts200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}

func (a *API) SearchCatalogAssets(ctx context.Context, request api.SearchCatalogAssetsRequestObject) (api.SearchCatalogAssetsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.SearchCatalogAssets401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.unauthenticated(ctx))}, nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.SearchCatalogAssets404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	q := ""
	if request.Params.Q != nil {
		q = *request.Params.Q
	}
	page, err := a.Catalog.SearchAssets(ctx, q, intValue(request.Params.Offset), intValue(request.Params.Limit))
	if err != nil {
		return nil, err
	}
	items := make([]api.CatalogAsset, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPICatalogAsset(&page.Items[i]))
	}
	return api.SearchCatalogAssets200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}

func (a *API) PatchObservation(ctx context.Context, request api.PatchObservationRequestObject) (api.PatchObservationResponseObject, error) {
	user, session, ok := a.currentSession(ctx)
	if !ok {
		return api.PatchObservation401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.PatchObservation404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Observations == nil || a.Jobs == nil {
		return nil, ErrNotImplemented
	}
	version, ok := observation.ParseETag(request.Params.IfMatch)
	if !ok {
		return api.PatchObservation412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	}
	if request.Body == nil {
		return api.PatchObservation400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", "A request body is required."))}, nil
	}
	in := toPatchInput(ctx, user.ID, request.Body)
	item, err := a.Observations.Patch(ctx, request.PortfolioId, request.ObservationId, version, &in)
	switch {
	case errors.Is(err, observation.ErrNotFound):
		return api.PatchObservation404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	case errors.Is(err, observation.ErrPrecondition):
		return api.PatchObservation412ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "The If-Match ETag does not match the current version.")), nil
	case errors.Is(err, observation.ErrInvalid), errors.Is(err, observation.ErrCoverageRequired):
		return api.PatchObservation400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
	case err != nil:
		return nil, err
	}
	job, err := a.Jobs.EnqueueRebuild(ctx, request.PortfolioId, &user.ID, "observation correction")
	if err != nil {
		return nil, err
	}
	etag := item.ETag()
	return api.PatchObservation200JSONResponse{
		Body:    api.ObservationCorrection{Observation: toAPIObservation(&item), Job: toAPIJob(&job, false)},
		Headers: api.PatchObservation200ResponseHeaders{ETag: etagPtr(etag)},
	}, nil
}

func toPatchInput(ctx context.Context, actor uuid.UUID, body *api.PatchObservationRequest) observation.PatchInput {
	in := observation.PatchInput{
		ActorUserID:       actor,
		RequestID:         requestTraceID(ctx),
		TotalQuantity:     body.TotalQuantity,
		AvailableQuantity: body.AvailableQuantity,
		LockedQuantity:    body.LockedQuantity,
		Quantity:          body.Quantity,
		EntryPrice:        body.EntryPrice,
		MarkPrice:         body.MarkPrice,
		UnrealizedValue:   body.UnrealizedValue,
		Leverage:          body.Leverage,
		Amount:            body.Amount,
		Price:             body.Price,
		DecimalValue:      body.DecimalValue,
		TextValue:         body.TextValue,
	}
	if body.Completeness != nil {
		value := string(*body.Completeness)
		in.Completeness = &value
	}
	if body.Side != nil {
		value := string(*body.Side)
		in.Side = &value
	}
	return in
}

func toAPIAccount(item *observation.Account) api.Account {
	return api.Account{
		AccountType: item.AccountType,
		DisplayName: item.DisplayName,
		ExternalRef: item.ExternalRef,
		Id:          item.ID,
		IdentityId:  item.IdentityID,
		PortfolioId: item.PortfolioID,
		Status:      api.AccountStatus(item.Status),
	}
}

func toAPICatalogAsset(item *catalog.Asset) api.CatalogAsset {
	return api.CatalogAsset{
		Code:     item.Code,
		Decimals: item.Decimals,
		Id:       item.ID,
		Kind:     api.CatalogAssetKind(item.Kind),
		Name:     item.Name,
		Status:   api.CatalogAssetStatus(item.Status),
	}
}

func toAPIObservation(item *observation.Observation) api.Observation {
	out := api.Observation{
		CollectedAt:        item.CollectedAt,
		Completeness:       api.ObservationCompleteness(item.Completeness),
		Id:                 item.ID,
		IdentityId:         item.IdentityID,
		IdentitySnapshotId: item.IdentitySnapshotID,
		Kind:               api.ObservationKind(item.Kind),
		PortfolioId:        item.PortfolioID,
		SourceObservedAt:   item.SourceObservedAt,
		Version:            int(item.Version),
	}
	if item.AccountID != nil {
		out.AccountId = item.AccountID
	}
	if item.Balance != nil {
		out.Balance = &api.BalanceObservation{
			AssetListingId:    item.Balance.AssetListingID,
			AvailableQuantity: item.Balance.AvailableQuantity,
			LockedQuantity:    item.Balance.LockedQuantity,
			TotalQuantity:     item.Balance.TotalQuantity,
		}
	}
	if item.Position != nil {
		out.Position = &api.PositionObservation{
			EntryPrice:          item.Position.EntryPrice,
			InstrumentId:        item.Position.InstrumentID,
			Leverage:            item.Position.Leverage,
			MarkPrice:           item.Position.MarkPrice,
			PlatformPositionRef: item.Position.PlatformPositionRef,
			Quantity:            item.Position.Quantity,
			Side:                api.PositionObservationSide(item.Position.Side),
			UnrealizedValue:     item.Position.UnrealizedValue,
		}
	}
	if item.ReportedEquity != nil {
		out.ReportedEquity = &api.ReportedEquityObservation{
			Amount:                item.ReportedEquity.Amount,
			CoveredAccountIds:     item.ReportedEquity.CoveredAccountIDs,
			CoveredIdentityIds:    item.ReportedEquity.CoveredIdentityIDs,
			DenominationListingId: item.ReportedEquity.DenominationListingID,
		}
	}
	if item.Price != nil {
		price := api.PriceObservation{
			Price:          item.Price.Price,
			PriceType:      api.PriceObservationPriceType(item.Price.PriceType),
			QuoteListingId: item.Price.QuoteListingID,
		}
		if item.Price.InstrumentID != nil {
			price.InstrumentId = item.Price.InstrumentID
		}
		if item.Price.BaseListingID != nil {
			price.BaseListingId = item.Price.BaseListingID
		}
		out.Price = &price
	}
	if item.AccountMetric != nil {
		out.AccountMetric = &api.AccountMetricObservation{
			DecimalValue:     item.AccountMetric.DecimalValue,
			MetricCode:       item.AccountMetric.MetricCode,
			SemanticsVersion: item.AccountMetric.SemanticsVersion,
			TextValue:        item.AccountMetric.TextValue,
			Unit:             item.AccountMetric.Unit,
		}
	}
	return out
}

func toOffsetPage(offset, limit, total int) api.OffsetPage {
	return api.OffsetPage{Kind: api.Offset, Offset: offset, Limit: limit, Total: total}
}

func intValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func requestTraceID(ctx context.Context) string {
	c := ginFrom(ctx)
	if c == nil {
		return "unknown"
	}
	if value, ok := c.Get("trace_id"); ok {
		if trace, ok := value.(string); ok && trace != "" {
			return trace
		}
	}
	return "unknown"
}
