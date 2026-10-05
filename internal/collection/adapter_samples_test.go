package collection

import (
	"path/filepath"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

func loadAdapterSamples(t *testing.T, platform adapters.Platform, selection string) []fixture.File {
	t.Helper()
	names := map[adapters.Platform][]string{adapters.PlatformBinance: {"api_v3_time.json", "api_v3_account.json", "api_v3_ticker_price.json", "sapi_v3_asset_getUserAsset.json", "sapi_v1_asset_get_funding_asset.json", "fapi_v1_exchangeInfo.json", "sapi_v1_dci_product_positions.pending.json", "sapi_v1_dci_product_positions.settling.json", "sapi_v1_dci_product_positions.refunding.json", "sapi_v1_dci_product_positions.purchase_success.json", "sapi_v1_simple_earn_flexible_position.json", "sapi_v1_simple_earn_locked_position.json", "sapi_v1_asset_wallet_balance.json"}, adapters.PlatformOKX: {"api_v5_public_time.json", "api_v5_asset_balances.json", "api_v5_finance_savings_balance.json", "api_v5_finance_staking_defi_orders_active.json", "api_v5_market_tickers.json", "api_v5_public_instruments.swap.json", "api_v5_public_instruments.futures.json", "api_v5_asset_asset_valuation.json", "api_v5_finance_sfp_dcd_order_history.json", "api_v5_account_config.pm.json", "api_v5_account_balance.pm.json", "api_v5_account_positions.pm.json"}, adapters.PlatformBybit: {"v5_market_time.json", "v5_user_query_api.json", "v5_asset_transfer_query_account_coins_balance.json", "v5_earn_position.json", "v5_earn_product.json", "v5_market_tickers.json", "v5_market_instruments_info.linear.json", "v5_market_instruments_info.inverse.json", "v5_position_list.option.json", "v5_account_info.pm.json", "v5_account_wallet_balance.pm.json", "v5_position_list.linear_usdt_pm.json", "v5_position_list.linear_usdc_pm.json", "v5_position_list.inverse_pm.json", "v5_earn_fixed_term_position.empty.json", "v5_earn_advance_position.sell_high.json", "v5_asset_asset_overview.json"}, adapters.PlatformKraken: {"0_public_Assets.json", "0_public_AssetPairs.json", "0_public_Ticker.json", "0_private_GetApiKeyInfo.json", "0_private_Earn_Strategies.json", "0_private_Earn_Allocations.json", "api_auth_v1_api_keys_v3_check.json", "derivatives_api_v3_openpositions.json", "derivatives_api_v3_instruments.json", "derivatives_api_v3_tickers.json", "0_private_BalanceEx.unified.json", "0_private_TradeBalance.unified.json", "ws_v1_balances_snapshot.unified.json"}}[platform]
	if platform == adapters.PlatformBinance {
		switch selection {
		case "binance_classic":
			names = append(names, "sapi_v1_account_info.classic.json", "sapi_v1_margin_account.json", "sapi_v1_margin_isolated_account.json", "fapi_v3_account.json", "fapi_v3_positionRisk.json", "fapi_v1_multiAssetsMargin.json", "fapi_v1_symbolConfig.json", "dapi_v1_account.json", "dapi_v1_positionRisk.json", "dapi_v1_exchangeInfo.json")
		case "binance_classic_single_asset":
			names = []string{
				"api_v3_time.json", "api_v3_account.json", "api_v3_ticker_price.json", "fapi_v1_exchangeInfo.json",
				"sapi_v1_account_info.classic.json", "fapi_v1_multiAssetsMargin.single_asset.json",
				"fapi_v3_account.single_asset.json", "fapi_v3_positionRisk.hedge.json", "fapi_v1_symbolConfig.json",
			}
		case "binance_classic_multi_assets":
			names = []string{
				"api_v3_time.json", "api_v3_account.json", "api_v3_ticker_price.json", "fapi_v1_exchangeInfo.json",
				"sapi_v1_account_info.classic.json", "fapi_v1_multiAssetsMargin.json",
				"fapi_v3_account.json", "fapi_v3_positionRisk.hedge.json", "fapi_v1_symbolConfig.json",
			}
		case "binance_classic_one_way_isolated":
			names = []string{
				"api_v3_time.json", "api_v3_account.json", "api_v3_ticker_price.json", "fapi_v1_exchangeInfo.json",
				"sapi_v1_account_info.classic.json", "fapi_v1_multiAssetsMargin.single_asset.json",
				"fapi_v3_account.one_way_isolated.json", "fapi_v3_positionRisk.one_way_isolated.json", "fapi_v1_symbolConfig.isolated.json",
			}
		case "binance_classic_coin_m":
			names = []string{
				"api_v3_time.json", "api_v3_account.json", "api_v3_ticker_price.json", "sapi_v1_account_info.classic.json",
				"dapi_v1_account.json", "dapi_v1_positionRisk.json", "dapi_v1_exchangeInfo.json",
			}
		case "":
			names = append(names, "sapi_v1_account_info.pm.json", "papi_v1_account.json", "papi_v1_balance.json", "papi_v1_um_positionRisk.json", "papi_v1_cm_positionRisk.json", "dapi_v1_exchangeInfo.json")
		default:
			t.Fatalf("unknown sample selection %s", selection)
		}
	} else if selection != "" {
		t.Fatalf("unknown sample selection %s", selection)
	}
	files, err := fixture.Load(filepath.Join("..", "adapters", string(platform), "testdata"), names...)
	if err != nil {
		t.Fatal(err)
	}
	return files
}
