package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/domain/catalog"
	"github.com/rbee3u/plutus/internal/domain/jobs"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func boolValue(value *bool) bool {
	return value != nil && *value
}

func (a *API) ListAssetListings(ctx context.Context, request api.ListAssetListingsRequestObject) (api.ListAssetListingsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Catalog.ListListings(ctx, catalog.ListFilter{PortfolioID: sc.portfolio.ID, Platform: stringValue(request.Params.Platform), Query: stringValue(request.Params.Q), IgnoredOnly: boolValue(request.Params.IgnoredOnly), Offset: intValue(request.Params.Offset), Limit: intValue(request.Params.Limit)})
	if err != nil {
		return nil, err
	}
	items := make([]api.AssetListing, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIListing(&page.Items[i]))
	}
	return api.ListAssetListings200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}

func (a *API) PutAssetListingMapping(ctx context.Context, request api.PutAssetListingMappingRequestObject) (api.PutAssetListingMappingResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	version, ok := catalog.ParseListingETag(request.Params.IfMatch)
	if !ok {
		return nil, validationError("If-Match must be a listing ETag.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	item, affected, err := a.Catalog.PutMapping(ctx, request.ListingId, catalog.MappingInput{
		ActorUserID: sc.user.ID,
		PortfolioID: sc.portfolio.ID,
		RequestID:   requestTraceID(ctx),
		AssetSymbol: request.Body.AssetSymbol,
		Version:     version,
	})
	if err != nil {
		return nil, err
	}
	if err := a.Catalog.WithExclusion(ctx, sc.portfolio.ID, &item); err != nil {
		return nil, err
	}
	if a.Jobs != nil {
		for _, portfolioID := range affected {
			if _, _, err := a.Jobs.EnqueueRecalc(ctx, portfolioID, jobs.RecalcPayload{Reason: "asset listing mapping changed", ListingID: item.ID.String()}, &sc.user.ID); err != nil {
				return nil, err
			}
		}
	}
	return api.PutAssetListingMapping200JSONResponse{Body: toAPIListing(&item), Headers: api.PutAssetListingMapping200ResponseHeaders{ETag: etagPtr(item.ETag())}}, nil
}

func (a *API) PutAssetListingExclusion(ctx context.Context, request api.PutAssetListingExclusionRequestObject) (api.PutAssetListingExclusionResponseObject, error) {
	sc, err := a.mutating(ctx, request.PortfolioId, request.Params.XCSRFToken)
	if err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	version, ok := catalog.ParseExclusionETag(request.Params.IfMatch)
	if !ok {
		return nil, validationError("If-Match must be a listing exclusion ETag.")
	}
	if request.Body == nil {
		return nil, validationError("A request body is required.")
	}
	item, err := a.Catalog.PutExclusion(ctx, request.ListingId, &catalog.ExclusionInput{
		PortfolioID: sc.portfolio.ID, ActorUserID: sc.user.ID, RequestID: requestTraceID(ctx),
		Ignored: request.Body.Ignored, Reason: stringValue(request.Body.Reason), Version: version,
	})
	if err != nil {
		return nil, err
	}
	return api.PutAssetListingExclusion200JSONResponse{Body: toAPIListing(&item), Headers: api.PutAssetListingExclusion200ResponseHeaders{ETag: etagPtr(item.ExclusionETag())}}, nil
}

func (a *API) ListInstruments(ctx context.Context, request api.ListInstrumentsRequestObject) (api.ListInstrumentsResponseObject, error) {
	if _, err := a.scoped(ctx, request.PortfolioId); err != nil {
		return nil, err
	}
	if a.Catalog == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Catalog.ListInstruments(ctx, catalog.ListFilter{Platform: stringValue(request.Params.Platform), Query: stringValue(request.Params.Q), Offset: intValue(request.Params.Offset), Limit: intValue(request.Params.Limit)})
	if err != nil {
		return nil, err
	}
	items := make([]api.Instrument, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIInstrument(&page.Items[i]))
	}
	return api.ListInstruments200JSONResponse{Items: items, Page: toOffsetPage(page.Offset, page.Limit, page.Total)}, nil
}
