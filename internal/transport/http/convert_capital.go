package httpserver

import (
	"github.com/rbee3u/plutus/internal/service/capital"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIFlow(f *capital.Flow) api.CapitalFlow {
	return api.CapitalFlow{
		Id:                   f.ID,
		PricingMethod:        api.CapitalFlowPricingMethod(f.PricingMethod),
		PricingPlatform:      f.PricingPlatform,
		PriceObservationId:   f.PriceObservationID,
		PricingAttemptedAt:   f.PricingAttemptedAt,
		CalculationVersion:   int(f.CalculationVersion), // #nosec G115 -- A compile-time policy version, currently 1; never user-supplied.
		PortfolioId:          f.PortfolioID,
		Direction:            api.CapitalFlowDirection(f.Direction),
		CanonicalAssetSymbol: f.CanonicalAssetSymbol,
		Quantity:             f.Quantity,
		ValueUsd:             f.ValueUSD,
		EffectiveAt:          f.EffectiveAt,
		AccountId:            f.AccountID,
		AccountName:          f.AccountName,
		IdentityId:           f.IdentityID,
		IdentityName:         f.IdentityName,
		Platform:             f.Platform,
		Note:                 f.Note,
		Version:              int(f.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		CreatedAt:            f.CreatedAt,
		UpdatedAt:            f.UpdatedAt,
	}
}
