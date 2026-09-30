package recording

import (
	"fmt"
	"strings"

	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

func marketData(path string) bool {
	for _, part := range []string{"exchangeInfo", "instruments", "tickers", "ticker/", "/Ticker", "/Assets", "/AssetPairs"} {
		if strings.Contains(path, part) {
			return true
		}
	}
	return false
}

// trimMarkets retains the default BTC/ETH/USD whitelist, explicitly requested symbols and
// representative contract types. It never trims private account or pagination responses.
func (s *Session) trimMarkets(values []any) {
	coins := map[string]bool{"BTC": true, "XBT": true, "XXBT": true, "ETH": true, "XETH": true, "USD": true, "ZUSD": true, "USDT": true, "USDC": true}
	contracts := map[string]bool{}
	for _, request := range []fixture.Request{s.request} {
		for _, params := range []map[string][]string{request.Query, request.Form} {
			for key, values := range params {
				if key == "symbol" || key == "instId" || key == "pair" || key == "asset" || key == "coin" || key == "ccy" {
					for _, value := range values {
						for _, symbol := range strings.Split(value, ",") {
							contracts[symbol], coins[symbol] = true, true
						}
					}
				}
			}
		}
	}
	pairs := map[string]bool{}
	for i := range s.files {
		rec := &s.files[i]
		if !strings.HasSuffix(rec.Request.Path, "/AssetPairs") {
			continue
		}
		root, ok := values[i].(map[string]any)
		if !ok {
			continue
		}
		rows, ok := root["result"].(map[string]any)
		if !ok {
			continue
		}
		for name, row := range rows {
			fields, ok := row.(map[string]any)
			if !ok {
				continue
			}
			base, _ := fields["base"].(string)
			quote, _ := fields["quote"].(string)
			if coins[base] && coins[quote] {
				pairs[name] = true
				if alt, ok := fields["altname"].(string); ok {
					pairs[alt] = true
				}
			}
		}
	}
	for i := range s.files {
		rec := &s.files[i]
		if !marketData(rec.Request.Path) {
			continue
		}
		before, after := 0, 0
		keepSymbol := func(symbol string) bool {
			if contracts[symbol] || coins[symbol] || pairs[symbol] {
				return true
			}
			for coin := range coins {
				for _, quote := range []string{"USD", "USDT", "USDC"} {
					if symbol == coin+quote || symbol == coin+"-"+quote || symbol == coin+"-"+quote+"-SWAP" {
						return true
					}
				}
			}
			return false
		}
		groups := map[string]bool{}
		filter := func(rows []any) []any {
			out := make([]any, 0, len(rows))
			for _, row := range rows {
				before++
				fields, ok := row.(map[string]any)
				if !ok {
					out = append(out, row)
					after++
					continue
				}
				symbol, _ := fields["symbol"].(string)
				if symbol == "" {
					symbol, _ = fields["instId"].(string)
				}
				keep := keepSymbol(symbol)
				var parts []string
				for _, key := range []string{"contractType", "ctType", "type"} {
					if v, ok := fields[key].(string); ok {
						parts = append(parts, v)
					}
				}
				group := strings.Join(parts, "/")
				if group != "" && !strings.Contains(strings.ToLower(group), "option") && !groups[group] {
					keep = true
					groups[group] = true
				}
				if keep {
					out = append(out, row)
					after++
				}
			}
			return out
		}
		switch root := values[i].(type) {
		case []any:
			values[i] = filter(root)
		case map[string]any:
			for _, key := range []string{"symbols", "data", "instruments", "tickers"} {
				if rows, ok := root[key].([]any); ok {
					root[key] = filter(rows)
				}
			}
			if result, ok := root["result"].(map[string]any); ok {
				if rows, ok := result["list"].([]any); ok {
					result["list"] = filter(rows)
				} else {
					for name, row := range result {
						before++
						keep := keepSymbol(name)
						if fields, ok := row.(map[string]any); ok {
							if alt, ok := fields["altname"].(string); ok {
								keep = keep || coins[alt]
							}
						}
						if keep {
							after++
						} else {
							delete(result, name)
						}
					}
				}
			}
		}
		if before > 0 {
			note := fmt.Sprintf("Kept %d of %d market rows: default whitelist, requested symbols and representative contract types. Business values are unchanged.", after, before)
			rec.Notes = note
		}
	}
}
