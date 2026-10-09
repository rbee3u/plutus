package binance

import (
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// Synthetic scenes intentionally choose stable values independently of recorded evidence.
func TestClassicSyntheticScenes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		selection                string
		account                  adapters.AccountType
		equity                   string
		multi, isolated, inverse bool
	}{
		{classicSingleAsset, TypeUMFutures, "101.5", false, false, false},
		{classicMultiAssets, TypeUMFutures, "101.5", true, false, false},
		{classicOneWayIsolated, TypeUMFutures, "101", false, true, false},
		{classicCoinM, TypeCMFutures, "0.011", false, false, true},
	} {
		t.Run(tc.selection, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t, tc.selection)
			report := reportOf(t, collect(t, a, adapters.StructureBinanceClassic, tc.account), tc.account)
			if report.Status != adapters.StatusOK || report.Equity != nil || report.UnifiedMarginRatio != nil {
				t.Fatalf("report %+v", report)
			}
			if tc.multi {
				if deref(report.AdjustedEquity) != "121.5" || deref(report.AvailableEquity) != "100" || deref(report.InitialMarginRatio) != "0.1" || deref(report.MaintenanceMarginRatio) != "0.02" || len(report.Balances) != 2 {
					t.Fatalf("Multi-Assets measures and coin balances %+v", report)
				}
			} else if report.AdjustedEquity != nil || report.AvailableEquity != nil || report.InitialMarginRatio != nil || report.MaintenanceMarginRatio != nil || len(report.Balances) != 1 {
				t.Fatalf("Single-Asset/COIN-M must omit USD account measures %+v", report)
			}
			symbol, available, withdraw, pnl := "USDT", "90", "80", "1.5"
			if tc.isolated {
				pnl = "1"
			}
			if tc.inverse {
				symbol, available, withdraw, pnl = "ETH", "0.008", "0.007", "0.001"
			}
			balance := balanceOf(t, report, symbol)
			if balance.Equity != tc.equity || deref(balance.AvailableToTrade) != available || deref(balance.AvailableToWithdraw) != withdraw || deref(balance.UnrealizedPnl) != pnl {
				t.Fatalf("coin measures %+v", balance)
			}
			want := map[string]struct{ side, quantity, entry, pnl, liquidation string }{
				"um:DOGEUSDT:LONG":  {adapters.SideLong, "100", "0.09", "1", "0.05"},
				"um:DOGEUSDT:SHORT": {adapters.SideShort, "50", "0.11", "0.5", "0.2"},
			}
			if tc.isolated {
				want = map[string]struct{ side, quantity, entry, pnl, liquidation string }{
					"um:DOGEUSDT:BOTH": {adapters.SideShort, "100", "0.11", "1", "0.12"},
				}
				rows := body[[]struct{ PositionSide, PositionAmt, IsolatedWallet, IsolatedMargin string }](t, r, epUMPositionRisk, nil)
				if len(rows) != 1 || rows[0].PositionSide != "BOTH" || rows[0].PositionAmt != "-100" || rows[0].IsolatedWallet != "2" || rows[0].IsolatedMargin != "3" {
					t.Fatalf("isolated response %+v", rows)
				}
			}
			if tc.inverse {
				want = map[string]struct{ side, quantity, entry, pnl, liquidation string }{
					"cm:ETHUSD_PERP:BOTH":   {adapters.SideLong, "2", "2000", "0.002", "1500"},
					"cm:ETHUSD_261225:BOTH": {adapters.SideShort, "1", "2000", "-0.001", "3500"},
				}
			}
			if len(report.Positions) != len(want) {
				t.Fatalf("positions %+v", report.Positions)
			}
			for _, p := range report.Positions {
				expected, ok := want[p.PlatformPositionRef]
				if !ok || p.Side != expected.side || p.Quantity != expected.quantity || deref(p.EntryPrice) != expected.entry || deref(p.UnrealizedPnl) != expected.pnl || deref(p.LiquidationPrice) != expected.liquidation || deref(p.Leverage) != "5" {
					t.Fatalf("position %+v", p)
				}
				if tc.inverse {
					kind := adapters.KindInversePerpetual
					if p.PlatformPositionRef == "cm:ETHUSD_261225:BOTH" {
						kind = adapters.KindInverseDelivery
						if p.FuturesContract.ExpiryAt == nil || !p.FuturesContract.ExpiryAt.Equal(time.UnixMilli(1798185600000).UTC()) {
							t.Fatalf("delivery expiry %+v", p.FuturesContract)
						}
					}
					if p.QuantityUnit != adapters.UnitContracts || p.FuturesContract.Kind != kind || p.FuturesContract.BaseSymbol != "ETH" || p.FuturesContract.QuoteSymbol != "USD" || p.FuturesContract.SettlementSymbol != "ETH" || deref(p.FuturesContract.ContractSize) != "10" || deref(p.MarkPrice) != "2500" {
						t.Fatalf("inverse terms %+v", p)
					}
				} else if p.QuantityUnit != adapters.UnitBase || p.FuturesContract.Kind != adapters.KindLinearPerpetual || p.FuturesContract.BaseSymbol != "DOGE" || p.FuturesContract.QuoteSymbol != "USDT" || p.FuturesContract.SettlementSymbol != "USDT" || deref(p.MarkPrice) != "0.1" {
					t.Fatalf("linear terms %+v", p)
				}
				delete(want, p.PlatformPositionRef)
			}
			if len(want) != 0 {
				t.Fatalf("missing positions %+v", want)
			}
			expectedCalls := []endpoint{epServerTime, epTickerPrice, epAccountInfo, epUMExchangeInfo, epUMMultiAssets, epUMAccount, epUMPositionRisk, epUMSymbolConfig}
			if tc.inverse {
				expectedCalls = []endpoint{epServerTime, epTickerPrice, epAccountInfo, epCMAccount, epCMPositionRisk, epCMExchangeInfo}
			}
			if len(r.called()) != len(expectedCalls) {
				t.Fatalf("unexpected requests %+v", r.called())
			}
			for _, ep := range expectedCalls {
				if r.count(ep) != 1 {
					t.Fatalf("%s consumed %d times", ep, r.count(ep))
				}
			}
		})
	}
}
