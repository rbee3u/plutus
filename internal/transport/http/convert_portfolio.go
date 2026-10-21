package httpserver

import (
	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIPortfolio(item *portfolio.Portfolio) api.Portfolio {
	return api.Portfolio{
		Id:                        item.ID,
		DeletionRequestedAt:       item.DeletionRequestedAt,
		Name:                      item.Name,
		Timezone:                  item.Timezone,
		CollectionIntervalSeconds: item.CollectionIntervalSeconds,
		Undeletable:               item.Undeletable,
		Version:                   int(item.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		CreatedAt:                 item.CreatedAt,
		UpdatedAt:                 item.UpdatedAt,
	}
}
