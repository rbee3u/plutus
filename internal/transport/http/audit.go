package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/domain/audit"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListAuditEvents(ctx context.Context, request api.ListAuditEventsRequestObject) (api.ListAuditEventsResponseObject, error) {
	sc, err := a.scoped(ctx, request.PortfolioId)
	if err != nil {
		return nil, err
	}
	if a.Audit == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Audit.List(ctx, sc.portfolio.ID, &audit.ListFilter{
		Action:       stringValue(request.Params.Action),
		ResourceType: stringValue(request.Params.ResourceType),
		Limit:        intValue(request.Params.Limit),
		Cursor:       stringValue(request.Params.Cursor),
	})
	if err != nil {
		return nil, err
	}
	items := make([]api.AuditEvent, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIAuditEvent(&page.Items[i]))
	}
	return api.ListAuditEvents200JSONResponse{Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore)}, nil
}
