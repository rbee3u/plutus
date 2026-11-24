package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListExposure(ctx context.Context, request api.ListExposureRequestObject) (api.ListExposureResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	filter := financial.ListFilter{
		CycleID:    request.Params.CycleId,
		Platform:   stringValue(request.Params.Platform),
		IdentityID: request.Params.IdentityId,
		Symbol:     stringValue(request.Params.Q),
		Sort:       stringValue(request.Params.Sort),
		Offset:     intValue(request.Params.Offset),
		Limit:      intValue(request.Params.Limit),
	}
	list, err := a.Financial.Exposure(ctx, sc.portfolioContext(), &filter)
	if err != nil {
		return nil, err
	}
	return api.ListExposure200JSONResponse(toAPIExposureList(list)), nil
}

func (a *API) GetExposureDetail(ctx context.Context, request api.GetExposureDetailRequestObject) (api.GetExposureDetailResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Financial == nil {
		return nil, ErrNotImplemented
	}
	filter := financial.ListFilter{
		CycleID:    request.Params.CycleId,
		Platform:   stringValue(request.Params.Platform),
		IdentityID: request.Params.IdentityId,
	}
	detail, err := a.Financial.ExposureDetail(ctx, sc.portfolioContext(), &filter, request.CanonicalAssetSymbol)
	if err != nil {
		return nil, err
	}
	return api.GetExposureDetail200JSONResponse(toAPIExposureDetail(detail)), nil
}
