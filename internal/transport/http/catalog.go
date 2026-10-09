package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListPlatformAssets(ctx context.Context, request api.ListPlatformAssetsRequestObject) (api.ListPlatformAssetsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Catalog.ListPlatformAssets(ctx, catalog.ListFilter{PortfolioID: sc.portfolio.ID, Platform: stringValue(request.Params.Platform), Query: stringValue(request.Params.Q), IgnoredOnly: boolValue(request.Params.IgnoredOnly), Offset: intValue(request.Params.Offset), Limit: intValue(request.Params.Limit)})
	if err != nil {
		return nil, err
	}
	items := make([]api.PlatformAsset, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIPlatformAsset(&page.Items[i]))
	}
	return api.ListPlatformAssets200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}

func (a *API) PutPlatformAssetMapping(ctx context.Context, request api.PutPlatformAssetMappingRequestObject) (api.PutPlatformAssetMappingResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	version, ok := catalog.ParsePlatformAssetETag(request.Params.IfMatch)
	if !ok {
		return nil, validationError("If-Match must be a platform asset ETag.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	item, err := a.Catalog.PutMapping(ctx, request.PlatformAssetId, catalog.MappingInput{
		ActorUserID: sc.user.ID,
		PortfolioID: sc.portfolio.ID,
		RequestID:   requestTraceID(ctx),
		AssetSymbol: request.Body.AssetSymbol,
		Version:     version,
	})
	if err != nil {
		return nil, err
	}
	return api.PutPlatformAssetMapping200JSONResponse{Body: toAPIPlatformAsset(&item), Headers: api.PutPlatformAssetMapping200ResponseHeaders{ETag: new(item.ETag())}}, nil
}

func (a *API) PutPlatformAssetExclusion(ctx context.Context, request api.PutPlatformAssetExclusionRequestObject) (api.PutPlatformAssetExclusionResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	version, ok := catalog.ParseExclusionETag(request.Params.IfMatch)
	if !ok {
		return nil, validationError("If-Match must be a platform asset exclusion ETag.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	item, err := a.Catalog.PutExclusion(ctx, request.PlatformAssetId, &catalog.ExclusionInput{
		PortfolioID: sc.portfolio.ID, ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx),
		Ignored: request.Body.Ignored, Reason: stringValue(request.Body.Reason), Version: version,
	})
	if err != nil {
		return nil, err
	}
	return api.PutPlatformAssetExclusion200JSONResponse{Body: toAPIPlatformAsset(&item), Headers: api.PutPlatformAssetExclusion200ResponseHeaders{ETag: new(item.ExclusionETag())}}, nil
}

func (a *API) ListFuturesContracts(ctx context.Context, request api.ListFuturesContractsRequestObject) (api.ListFuturesContractsResponseObject, error) {
	if _, err := a.scoped(ctx, request.PortfolioId); err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Catalog.ListFuturesContracts(ctx, catalog.ListFilter{Platform: stringValue(request.Params.Platform), Query: stringValue(request.Params.Q), Offset: intValue(request.Params.Offset), Limit: intValue(request.Params.Limit)})
	if err != nil {
		return nil, err
	}
	items := make([]api.FuturesContract, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIFuturesContract(&page.Items[i]))
	}
	return api.ListFuturesContracts200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}
