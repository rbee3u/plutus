package recording

import (
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strings"
)

// Only known balance/position fields participate. Unknown fields, nulls and empty strings keep
// their whole row, so a new exchange shape cannot silently disappear during evidence capture.
var (
	zeroAmounts  = strings.Fields("free locked freeze withdrawing borrowed interest netAsset totalAsset totalWalletBalance crossMarginFree crossMarginLocked crossMarginBorrowed crossMarginInterest umWalletBalance umUnrealizedPNL cmWalletBalance cmUnrealizedPNL negativeBalance walletBalance unrealizedProfit marginBalance maintMargin initialMargin positionInitialMargin openOrderInitialMargin maxWithdrawAmount crossWalletBalance crossUnPnl availableBalance positionAmt entryPrice breakEvenPrice unRealizedProfit liquidationPrice isolatedMargin isolatedWallet notional notionalValue pos eq cashBal availBal availEq liab upl eqUsd disEq frozenBal ordFrozen uplLiab isoEq isoUpl imr mmr margin avgPx liqPx posCcy posImr posMmr size positionIM positionMM positionBalance unrealisedPnl cumRealisedPnl equity usdValue accruedInterest borrowAmount spotBorrow bonus totalOrderIM totalPositionIM totalPositionMM spotHedgingQty availableToWithdraw balance credit credit_used hold_trade quantity available")
	zeroMetadata = strings.Fields("asset coin ccy currency symbol instId instType posId positionSide posSide marginAsset marginType mgnMode isolated marginMode tradeMode leverage lever maxQty maxNotionalValue markPrice markPx positionIdx side contractType unit updateTime uTime cTime createdTime updatedTime isAutoAddMargin autoAddMargin isReduceOnly collateralSwitch marginCollateral marginAvailable borrowEnabled repayEnabled status positionStatus riskId riskLimitValue settleCoin availableToBorrow")
	zeroRequired = strings.Fields("free totalWalletBalance walletBalance cashBal eq equity balance positionAmt pos size quantity")
)

func zeroNumber(value any) bool {
	var text string
	switch value := value.(type) {
	case string:
		text = value
	case json.Number:
		text = string(value)
	default:
		return false
	}
	if text == "" {
		return false
	}
	number, ok := new(big.Rat).SetString(text)
	return ok && number.Sign() == 0
}

func zeroShape(row map[string]any) (string, bool) {
	found := false
	keys := make([]string, 0, len(row))
	for key, value := range row {
		keys = append(keys, key+":"+fmt.Sprintf("%T", value))
		if slices.Contains(zeroAmounts, key) {
			if !zeroNumber(value) {
				return "", false
			}
			found = found || slices.Contains(zeroRequired, key)
			continue
		}
		if !slices.Contains(zeroMetadata, key) {
			return "", false
		}
		if value == "" {
			return "", false
		}
		switch value.(type) {
		case string, json.Number, bool:
		default:
			return "", false
		}
		switch key {
		case "status", "positionStatus":
			if value != "Normal" && value != "NORMAL" && value != "normal" {
				return "", false
			}
		case "isAutoAddMargin", "autoAddMargin", "isReduceOnly":
			if value == true || value == "true" || value == "1" || value == json.Number("1") {
				return "", false
			}
		}
	}
	if !found {
		return "", false
	}
	slices.Sort(keys)
	var shape strings.Builder
	shape.WriteString(strings.Join(keys, ","))
	for _, key := range []string{"positionSide", "posSide", "marginType", "mgnMode", "isolated", "instType", "unit", "side", "positionIdx", "status", "positionStatus", "collateralSwitch", "marginCollateral", "marginAvailable", "borrowEnabled", "repayEnabled"} {
		if value, ok := row[key]; ok {
			raw, err := json.Marshal(value)
			if err != nil {
				return "", false
			}
			shape.WriteString(";" + key + "=" + string(raw))
		}
	}
	return shape.String(), true
}

// Each allowed interface is known to contain balance or position rows. Product lists and
// configuration responses are excluded even when their field names resemble a wallet.
func privateRows(path string) bool {
	switch path {
	case "/api/v3/account", "/sapi/v3/asset/getUserAsset", "/sapi/v1/asset/get-funding-asset",
		"/papi/v1/balance", "/papi/v1/um/positionRisk", "/papi/v1/cm/positionRisk",
		"/fapi/v3/account", "/fapi/v3/positionRisk", "/dapi/v1/account", "/dapi/v1/positionRisk",
		"/sapi/v1/margin/account", "/sapi/v1/margin/isolated/account",
		"/api/v5/asset/balances", "/api/v5/account/balance", "/api/v5/account/positions",
		"/v5/account/wallet-balance", "/v5/position/list", "/v5/asset/transfer/query-account-coins-balance",
		"/0/private/BalanceEx", "/derivatives/api/v3/openpositions", "/ws/v1/balances_snapshot":
		return true
	}
	return false
}

func (s *Session) trimZeroRows(values []any) {
	for i := range s.files {
		f := &s.files[i]
		if marketData(f.Request.Path) || !privateRows(f.Request.Path) {
			continue
		}
		before, after := 0, 0
		filter := func(rows []any) []any {
			seen := map[string]bool{}
			out := make([]any, 0, len(rows))
			for _, row := range rows {
				before++
				fields, ok := row.(map[string]any)
				if ok {
					shape, zero := zeroShape(fields)
					if zero && seen[shape] {
						continue
					}
					if zero {
						seen[shape] = true
					}
				}
				out = append(out, row)
				after++
			}
			return out
		}
		var walk func(any) any
		walk = func(value any) any {
			switch root := value.(type) {
			case []any:
				rows := filter(root)
				for j := range rows {
					rows[j] = walk(rows[j])
				}
				return rows
			case map[string]any:
				// Kraken BalanceEx and WS currency dictionaries have arbitrary asset keys.
				for _, key := range []string{"result", "currencies", "holding"} {
					rows, ok := root[key].(map[string]any)
					if !ok {
						continue
					}
					names := slices.Sorted(maps.Keys(rows))
					seen := map[string]bool{}
					for _, name := range names {
						shape, zero := "", false
						if fields, ok := rows[name].(map[string]any); ok {
							shape, zero = zeroShape(fields)
						} else if key == "holding" {
							zero = zeroNumber(rows[name])
							shape = "scalar"
						}
						if !zero {
							continue
						}
						before++
						if seen[shape] {
							delete(rows, name)
						} else {
							seen[shape] = true
							after++
						}
					}
				}
				for key, child := range root {
					root[key] = walk(child)
				}
			}
			return value
		}
		values[i] = walk(values[i])
		if before > after {
			appendNote(&f.Notes, fmt.Sprintf("Kept %d of %d inspected balance/position rows: omitted only redundant known zero shapes, retaining a representative of each shape and setting. Unknown fields, nulls, empty strings and all nonzero financial measures are retained; original totals are unchanged.", after, before))
		}
	}
}
