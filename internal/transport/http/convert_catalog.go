package httpserver

import (
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIAsset(asset *catalog.Asset) api.Asset {
	return api.Asset{
		Id:                   asset.ID,
		Platform:             asset.Platform,
		Symbol:               asset.Symbol,
		CanonicalAssetSymbol: asset.CanonicalAssetSymbol,
		MappingSource:        api.AssetMappingSource(asset.MappingSource),
		FirstObservedAt:      asset.FirstObservedAt,
		LastObservedAt:       asset.LastObservedAt,
		MappingVersion:       int(asset.MappingVersion), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		Excluded:             asset.Excluded,
		ExclusionReason:      asset.ExclusionReason,
		ExcludedAt:           asset.ExcludedAt,
		ExclusionVersion:     int(asset.ExclusionVersion), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
	}
}

func toAPIFuturesContract(contract *catalog.FuturesContract) api.FuturesContract {
	return api.FuturesContract{
		Id:                    contract.ID,
		Platform:              contract.Platform,
		Symbol:                contract.Symbol,
		Kind:                  contract.Kind,
		BaseAssetSymbol:       contract.BaseAssetSymbol,
		QuoteAssetSymbol:      contract.QuoteAssetSymbol,
		SettlementAssetSymbol: contract.SettlementAssetSymbol,
		ExpiryAt:              contract.ExpiryAt,
		ContractSize:          contract.ContractSize,
		FirstObservedAt:       contract.FirstObservedAt,
		LastObservedAt:        contract.LastObservedAt,
	}
}
