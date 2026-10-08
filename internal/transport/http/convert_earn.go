package httpserver

import (
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIEarnAmounts(in []adapters.EarnAmount) []api.EarnAmount {
	out := make([]api.EarnAmount, 0, len(in))
	for _, a := range in {
		out = append(out, api.EarnAmount{Symbol: a.Symbol, Amount: a.Amount, AssetSymbol: a.AssetSymbol})
	}
	return out
}

func toAPIEarnPositions(in []financial.EarnPositionRow) []api.EarnPositionRow {
	out := make([]api.EarnPositionRow, 0, len(in))
	for i := range in {
		r := &in[i]
		rates := make([]api.EarnRate, 0, len(r.Rates))
		rewards := make([]api.EarnReward, 0, len(r.Rewards))
		details := make([]api.EarnDetail, 0, len(r.Details))
		for _, rate := range r.Rates {
			rates = append(rates, api.EarnRate{
				Value:     rate.Value,
				Upper:     rate.Upper,
				Kind:      rate.Kind,
				Qualifier: rate.Qualifier,
				Label:     rate.Label,
				Symbol:    rate.Symbol,
			})
		}
		for _, reward := range r.Rewards {
			rewards = append(rewards, api.EarnReward{Symbol: reward.Symbol, Amount: reward.Amount, Kind: reward.Kind})
		}
		for _, detail := range r.Details {
			details = append(details, api.EarnDetail{Label: detail.Label, Value: detail.Value, At: detail.At})
		}
		out = append(out, api.EarnPositionRow{
			AccountId:           r.AccountID,
			AccountName:         r.AccountName,
			AccountType:         r.AccountType,
			IdentityId:          r.IdentityID,
			IdentityName:        r.IdentityName,
			Platform:            r.Platform,
			PlatformPositionRef: r.PlatformPositionRef,
			ProductRef:          r.ProductRef,
			ProductName:         r.ProductName,
			Type:                r.Type,
			Granularity:         r.Granularity,
			AmountBasis:         r.AmountBasis,
			Assets:              toAPIEarnAmounts(r.Assets),
			Rates:               rates,
			Rewards:             rewards,
			Details:             details,
			SubscribedAt:        r.SubscribedAt,
			MaturityAt:          r.MaturityAt,
			Status:              r.Status,
			Freshness:           api.Freshness(r.Freshness),
			ReportObservedAt:    r.ReportObservedAt,
			Issues:              toAPIIssues(r.Issues),
		})
	}
	return out
}

func toAPIEarnSummaries(in []financial.EarnBalanceSummary) []api.EarnBalanceSummary {
	out := make([]api.EarnBalanceSummary, 0, len(in))
	for _, r := range in {
		out = append(out, api.EarnBalanceSummary{
			AccountId:        r.AccountID,
			AccountName:      r.AccountName,
			IdentityName:     r.IdentityName,
			Platform:         r.Platform,
			Assets:           toAPIEarnAmounts(r.Assets),
			Freshness:        api.Freshness(r.Freshness),
			ReportObservedAt: r.ReportObservedAt,
			Issues:           toAPIIssues(r.Issues),
		})
	}
	return out
}
