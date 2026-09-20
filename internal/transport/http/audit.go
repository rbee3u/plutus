package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListAuditEvents(ctx context.Context, request api.ListAuditEventsRequestObject) (api.ListAuditEventsResponseObject, error) {
	user, _, ok := a.currentSession(ctx)
	if !ok {
		return api.ListAuditEvents401ApplicationProblemPlusJSONResponse(a.unauthenticated(ctx)), nil
	}
	if _, ok := a.ownedPortfolio(ctx, user.ID, request.PortfolioId); !ok {
		return api.ListAuditEvents404ApplicationProblemPlusJSONResponse(a.notFound(ctx)), nil
	}
	if a.Audit == nil {
		return nil, ErrNotImplemented
	}
	page, err := a.Audit.List(ctx, request.PortfolioId, &audit.ListFilter{
		Action:       stringValue(request.Params.Action),
		ResourceType: stringValue(request.Params.ResourceType),
		Limit:        intValue(request.Params.Limit),
		Cursor:       stringValue(request.Params.Cursor),
	})
	if err != nil {
		if errors.Is(err, audit.ErrInvalidPage) || errors.Is(err, audit.ErrInvalidCursor) {
			return api.ListAuditEvents400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
		}
		return nil, err
	}
	items := make([]api.AuditEvent, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, toAPIAuditEvent(&page.Items[i]))
	}
	return api.ListAuditEvents200JSONResponse{Items: items, Page: toCursorPage(page.Limit, page.NextCursor, page.HasMore)}, nil
}

func toAPIAuditEvent(item *audit.Event) api.AuditEvent {
	out := api.AuditEvent{
		Action:       item.Action,
		After:        item.After,
		Before:       item.Before,
		Id:           item.ID,
		Metadata:     item.Metadata,
		OccurredAt:   item.OccurredAt,
		PortfolioId:  item.PortfolioID,
		ResourceType: item.ResourceType,
	}
	if item.ActorUserID != nil {
		out.ActorUserId = item.ActorUserID
	}
	resourceID := item.ResourceID
	out.ResourceId = &resourceID
	if item.RequestID != "" {
		out.RequestId = &item.RequestID
	}
	return out
}
