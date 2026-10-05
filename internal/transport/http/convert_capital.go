package httpserver

import (
	"github.com/rbee3u/plutus/internal/domain/capital"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIFlow(f *capital.Flow) api.CapitalFlow {
	return api.CapitalFlow{
		Id: f.ID, PortfolioId: f.PortfolioID, Direction: api.CapitalFlowDirection(f.Direction), AssetSymbol: f.AssetSymbol,
		Quantity: f.Quantity, ValueUsd: f.ValueUSD, EffectiveAt: f.EffectiveAt, AccountId: f.AccountID, AccountName: f.AccountName,
		Note: f.Note, Version: int(f.Version), CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt, // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
	}
}
