package httpserver

import (
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIPlatformAsset(l *catalog.PlatformAsset) api.PlatformAsset {
	return api.PlatformAsset{
		Id:               l.ID,
		Platform:         l.Platform,
		Symbol:           l.Symbol,
		AssetSymbol:      l.AssetSymbol,
		Name:             l.Name,
		MappingSource:    api.PlatformAssetMappingSource(l.MappingSource),
		FirstSeenAt:      l.FirstSeenAt,
		LastSeenAt:       l.LastSeenAt,
		Version:          int(l.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		Ignored:          l.Ignored,
		IgnoreReason:     l.IgnoreReason,
		IgnoredAt:        l.IgnoredAt,
		ExclusionVersion: int(l.ExclusionVersion), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
	}
}

func toAPIFuturesContract(i *catalog.FuturesContract) api.FuturesContract {
	return api.FuturesContract{
		Id:               i.ID,
		Platform:         i.Platform,
		Symbol:           i.Symbol,
		Kind:             i.Kind,
		BaseSymbol:       i.BaseSymbol,
		QuoteSymbol:      i.QuoteSymbol,
		SettlementSymbol: i.SettlementSymbol,
		ExpiryAt:         i.ExpiryAt,
		ContractSize:     i.ContractSize,
		FirstSeenAt:      i.FirstSeenAt,
		LastSeenAt:       i.LastSeenAt,
	}
}
