package httpserver

import (
	"github.com/rbee3u/plutus/internal/application/capital"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIFlow(f *capital.Flow) api.CapitalFlow {
	return api.CapitalFlow{
		Id:                   f.ID,
		PortfolioId:          f.PortfolioID,
		Direction:            api.CapitalFlowDirection(f.Direction),
		CanonicalAssetSymbol: f.CanonicalAssetSymbol,
		Quantity:             f.Quantity,
		ValueUsd:             f.ValueUSD,
		EffectiveAt:          f.EffectiveAt,
		AccountId:            f.AccountID,
		AccountName:          f.AccountName,
		Note:                 f.Note,
		Version:              int(f.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		CreatedAt:            f.CreatedAt,
		UpdatedAt:            f.UpdatedAt,
	}
}
