package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListAssets(ctx context.Context, request api.ListAssetsRequestObject) (api.ListAssetsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	status := ""
	if request.Params.Status != nil {
		if !request.Params.Status.Valid() {
			return nil, validationError("Status must be all, active, or excluded.")
		}
		status = string(*request.Params.Status)
	}
	page, err := a.Catalog.ListAssets(ctx, catalog.ListFilter{
		PortfolioID: sc.portfolio.ID,
		Platform:    stringValue(request.Params.Platform),
		Query:       stringValue(request.Params.Q),
		Status:      status,
		Offset:      intValue(request.Params.Offset),
		Limit:       intValue(request.Params.Limit),
	})
	if err != nil {
		return nil, err
	}
	items := make([]api.Asset, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIAsset(&page.Items[i]))
	}
	return api.ListAssets200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}

func (a *API) PutAssetMapping(ctx context.Context, request api.PutAssetMappingRequestObject) (api.PutAssetMappingResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	version, ok := catalog.ParseAssetETag(request.Params.IfMatch)
	if !ok {
		return nil, validationError("If-Match must be an asset ETag.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	item, err := a.Catalog.PutMapping(ctx, request.AssetId, catalog.MappingInput{
		ActorUserID:          sc.user.ID,
		PortfolioID:          sc.portfolio.ID,
		RequestID:            requestTraceID(ctx),
		CanonicalAssetSymbol: request.Body.CanonicalAssetSymbol,
		MappingVersion:       version,
	})
	if err != nil {
		return nil, err
	}
	return api.PutAssetMapping200JSONResponse{Body: toAPIAsset(&item), Headers: api.PutAssetMapping200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) PutAssetExclusion(ctx context.Context, request api.PutAssetExclusionRequestObject) (api.PutAssetExclusionResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	version, ok := catalog.ParseExclusionETag(request.Params.IfMatch)
	if !ok {
		return nil, validationError("If-Match must be an asset exclusion ETag.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	item, err := a.Catalog.PutExclusion(ctx, request.AssetId, &catalog.ExclusionInput{
		PortfolioID: sc.portfolio.ID, ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx),
		Excluded: request.Body.Excluded, Reason: stringValue(request.Body.Reason), Version: version,
	})
	if err != nil {
		return nil, err
	}
	return api.PutAssetExclusion200JSONResponse{Body: toAPIAsset(&item), Headers: api.PutAssetExclusion200ResponseHeaders{ETag: new(item.ExclusionETag())}}, nil
}

func (a *API) ListFuturesContracts(ctx context.Context, request api.ListFuturesContractsRequestObject) (api.ListFuturesContractsResponseObject, error) {
	if _, err := a.scoped(ctx, request.PortfolioId); err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Catalog.ListFuturesContracts(ctx, catalog.ListFilter{PortfolioID: request.PortfolioId, Platform: stringValue(request.Params.Platform), Query: stringValue(request.Params.Q), Offset: intValue(request.Params.Offset), Limit: intValue(request.Params.Limit)})
	if err != nil {
		return nil, err
	}
	items := make([]api.FuturesContract, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIFuturesContract(&page.Items[i]))
	}
	return api.ListFuturesContracts200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}
