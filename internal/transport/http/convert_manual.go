package httpserver

import (
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIManual(state manual.State) api.ManualBalanceState {
	convert := func(items []manual.Balance) []api.ManualBalance {
		rows := make([]api.ManualBalance, 0, len(items))
		for _, item := range items {
			rows = append(rows, api.ManualBalance{
				PlatformAssetSymbol: item.PlatformAssetSymbol,
				PlatformAssetId:     item.PlatformAssetID,
				AssetSymbol:         item.AssetSymbol,
				Quantity:            item.Quantity,
				ModifiedAt:          item.ModifiedAt,
			})
		}
		return rows
	}
	return api.ManualBalanceState{
		AccountId:      state.AccountID,
		IdentityId:     state.IdentityID,
		Version:        state.Version,
		AppliedVersion: state.AppliedVersion,
		Items:          convert(state.Items),
		AppliedItems:   convert(state.AppliedItems),
		Pending:        state.Pending,
	}
}
