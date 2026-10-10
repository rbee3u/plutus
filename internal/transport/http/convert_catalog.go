package httpserver

import (
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIAsset(asset *catalog.Asset) api.Asset {
	return api.Asset{
		Id:               asset.ID,
		Platform:         asset.Platform,
		Symbol:           asset.Symbol,
		CanonicalSymbol:  asset.CanonicalSymbol,
		Name:             asset.Name,
		MappingSource:    api.AssetMappingSource(asset.MappingSource),
		FirstSeenAt:      asset.FirstSeenAt,
		LastSeenAt:       asset.LastSeenAt,
		Version:          int(asset.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		Ignored:          asset.Ignored,
		IgnoreReason:     asset.IgnoreReason,
		IgnoredAt:        asset.IgnoredAt,
		ExclusionVersion: int(asset.ExclusionVersion), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
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
		FirstSeenAt:           contract.FirstSeenAt,
		LastSeenAt:            contract.LastSeenAt,
	}
}
