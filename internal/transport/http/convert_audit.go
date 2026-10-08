package httpserver

import (
	"github.com/rbee3u/plutus/internal/application/audit"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIAuditEvent(item *audit.Event) api.AuditEvent {
	return api.AuditEvent{
		Action:       item.Action,
		After:        item.After,
		Before:       item.Before,
		Id:           item.ID,
		Metadata:     item.Metadata,
		OccurredAt:   item.OccurredAt,
		PortfolioId:  item.PortfolioID,
		ResourceType: item.ResourceType,
		ActorUserId:  item.ActorUserID,
		ResourceId:   new(item.ResourceID),
		RequestId:    nonEmptyStringPtr(item.RequestID),
	}
}
