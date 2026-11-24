package httpserver

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIExposureList(list financial.ExposureList) api.ExposureList {
	items := make([]api.ExposureRow, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, toAPIExposureRow(&list.Items[i]))
	}
	return api.ExposureList{
		Context:  toAPIContext(&list.Context),
		Summary:  toAPIExposureSummary(list.Summary),
		Coverage: toAPIExposureCoverage(list.Coverage),
		Items:    items,
		Page:     toOffsetPage(list.Offset, list.Limit, list.Total),
		Issues:   toAPIIssues(list.Issues),
	}
}

func toAPIExposureSummary(summary financial.ExposureSummary) api.ExposureSummary {
	return api.ExposureSummary{
		AssetCount:   summary.AssetCount,
		LongCount:    summary.LongCount,
		ShortCount:   summary.ShortCount,
		FlatCount:    summary.FlatCount,
		PartialCount: summary.PartialCount,
		UnknownCount: summary.UnknownCount,
		Completeness: api.Completeness(summary.Completeness),
		Freshness:    api.Freshness(summary.Freshness),
	}
}

func toAPIExposureCoverage(coverage financial.ExposureCoverage) api.ExposureCoverage {
	ids := coverage.MissingAccountIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return api.ExposureCoverage{
		Complete:                  coverage.Complete,
		MissingAccountIds:         ids,
		MissingAccountCount:       coverage.MissingAccountCount,
		StaleAccountCount:         coverage.StaleAccountCount,
		ExcludedAssetCount:        coverage.ExcludedAssetCount,
		UnattributedPositionCount: coverage.UnattributedPositionCount,
	}
}

func toAPIExposureRow(row *financial.ExposureRow) api.ExposureRow {
	symbols := row.PlatformSymbols
	if symbols == nil {
		symbols = []string{}
	}
	return api.ExposureRow{
		CanonicalAssetSymbol: row.CanonicalAssetSymbol,
		PlatformSymbols:      symbols,
		NetQuantity:          row.NetQuantity,
		State:                row.State,
		PriceUsd:             row.PriceUSD,
		PriceSource:          nonEmptyStringPtr(row.PriceSource),
		PriceSourceSymbol:    row.PriceSourceSymbol,
		PriceSourcePlatform:  nonEmptyStringPtr(row.PriceSourcePlatform),
		ValueUsd:             row.ValueUSD,
		AccountCount:         row.AccountCount,
		UnknownAccountCount:  row.UnknownAccountCount,
		Freshness:            api.Freshness(row.Freshness),
		Issues:               toAPIIssues(row.Issues),
	}
}

func toAPIExposureDetail(detail financial.ExposureDetail) api.ExposureDetail {
	symbols := detail.PlatformSymbols
	if symbols == nil {
		symbols = []string{}
	}
	contributions := make([]api.ExposureContribution, 0, len(detail.Contributions))
	for i := range detail.Contributions {
		c := &detail.Contributions[i]
		contributions = append(contributions, api.ExposureContribution{
			Platform:         c.Platform,
			PlatformLabel:    c.PlatformLabel,
			IdentityId:       c.IdentityID,
			IdentityName:     c.IdentityName,
			AccountId:        c.AccountID,
			AccountName:      c.AccountName,
			AccountType:      c.AccountType,
			AccountStructure: c.AccountStructure,
			AssetSymbol:      c.AssetSymbol,
			Quantity:         c.Quantity,
			State:            c.State,
			ContractOnly:     c.ContractOnly,
			Freshness:        api.Freshness(c.Freshness),
			ReportObservedAt: c.ReportObservedAt,
			Issues:           toAPIIssues(c.Issues),
		})
	}
	return api.ExposureDetail{
		Context:              toAPIContext(&detail.Context),
		CanonicalAssetSymbol: detail.CanonicalAssetSymbol,
		PlatformSymbols:      symbols,
		NetQuantity:          detail.NetQuantity,
		State:                detail.State,
		AccountCount:         detail.AccountCount,
		UnknownAccountCount:  detail.UnknownAccountCount,
		Freshness:            api.Freshness(detail.Freshness),
		Coverage:             toAPIExposureCoverage(detail.Coverage),
		Contributions:        contributions,
		Issues:               toAPIIssues(detail.Issues),
	}
}
