package httpserver

import (
	"github.com/rbee3u/plutus/internal/domain/audit"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

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
