package recording

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

func marketData(path string) bool {
	for _, part := range []string{"exchangeInfo", "instruments", "tickers", "ticker/", "/Ticker", "/Assets", "/AssetPairs"} {
		if strings.Contains(path, part) {
			return true
		}
	}
	return false
}

func appendNote(fileNotes *string, note string) {
	*fileNotes = strings.TrimSpace(*fileNotes + " " + note)
}

func marketSymbol(row map[string]any) string {
	for _, key := range []string{"symbol", "instId", "altname"} {
		if value, ok := row[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

// marketGroup separates product kind, settlement, quote and quantity conventions. Base coin is
// deliberately not a group: recording every base coin would defeat representative selection.
func marketGroup(row map[string]any) string {
	parts := []string{}
	keys := make([]string, 0, len(row))
	for key, value := range row {
		keys = append(keys, fmt.Sprintf("%s:%T", key, value))
	}
	slices.Sort(keys)
	parts = append(parts, strings.Join(keys, ","))
	base := ""
	for _, key := range []string{"baseAsset", "baseCoin", "baseCcy", "base"} {
		if value, ok := row[key].(string); ok && value != "" {
			base = value
			break
		}
	}
	if base == "" {
		base, _, _ = strings.Cut(marketSymbol(row), "-")
	}
	for _, key := range []string{"contractType", "instType", "ctType", "ctValCcy", "type", "settleCoin", "settleCcy", "marginAsset", "quoteAsset", "quoteCoin", "quoteCcy", "quote", "contractSize", "contract_size"} {
		if value, ok := row[key]; ok {
			if key == "marginAsset" || key == "settleCoin" || key == "settleCcy" || key == "ctValCcy" {
				if currency, ok := value.(string); ok && base != "" && currency == base {
					value = "base"
				}
			}
			raw, err := json.Marshal(value)
			if err != nil {
				return ""
			}
			parts = append(parts, key+"="+string(raw))
		}
	}
	return strings.Join(parts, ";")
}

func preferredMarket(symbol string, row map[string]any) bool {
	for _, key := range []string{"baseAsset", "baseCoin", "baseCcy", "base"} {
		if value, ok := row[key].(string); ok {
			switch value {
			case "BTC", "XBT", "XXBT", "ETH", "XETH":
				return true
			}
		}
	}
	for _, coin := range []string{"BTC", "XBT", "XXBT", "ETH", "XETH", "USD", "ZUSD", "USDT", "USDC"} {
		if symbol == coin || strings.HasPrefix(symbol, coin+"USD") || strings.HasPrefix(symbol, coin+"-USD") || strings.HasPrefix(symbol, coin+"ZUSD") {
			return true
		}
	}
	return false
}

// trimMarkets selects representative whole rows; prepared symbols take priority without an
// additional account request. Unknown row shapes are kept rather than silently discarded.
func (s *Session) trimMarkets(values []any) {
	keep := map[string]bool{}
	for symbol := range s.keepSymbols {
		keep[symbol] = true
	}
	for _, params := range []map[string][]string{s.request.Query, s.request.Form} {
		for _, key := range []string{"symbol", "instId", "pair", "asset", "coin", "ccy"} {
			for _, value := range params[key] {
				for _, symbol := range strings.Split(value, ",") {
					keep[symbol] = true
				}
			}
		}
	}
	for i := range s.files {
		f := &s.files[i]
		if !marketData(f.Request.Path) {
			continue
		}
		before, after := 0, 0
		selectRows := func(rows []any, names []string) map[int]bool {
			selected := map[int]bool{}
			groups := map[string]int{}
			for j, value := range rows {
				before++
				row, ok := value.(map[string]any)
				if !ok {
					selected[j] = true
					continue
				}
				symbol := marketSymbol(row)
				if symbol == "" && len(names) > 0 {
					symbol = names[j]
				}
				if symbol == "" {
					selected[j] = true
					continue
				}
				if keep[symbol] || (len(names) > 0 && keep[names[j]]) {
					selected[j] = true
				}
				group := marketGroup(row)
				if group == "" {
					selected[j] = true
					continue
				}
				previous, exists := groups[group]
				if !exists {
					groups[group] = j
					continue
				}
				previousRow, _ := rows[previous].(map[string]any)
				previousSymbol := marketSymbol(previousRow)
				if previousSymbol == "" && len(names) > 0 {
					previousSymbol = names[previous]
				}
				if preferredMarket(symbol, row) && !preferredMarket(previousSymbol, previousRow) {
					groups[group] = j
				}
			}
			for _, j := range groups {
				selected[j] = true
			}
			after += len(selected)
			return selected
		}
		filter := func(rows []any) []any {
			selected := selectRows(rows, nil)
			out := make([]any, 0, len(selected))
			for j, row := range rows {
				if selected[j] {
					out = append(out, row)
				}
			}
			return out
		}
		filterMap := func(rows map[string]any) {
			names := slices.Sorted(maps.Keys(rows))
			list := make([]any, 0, len(rows))
			for _, name := range names {
				list = append(list, rows[name])
			}
			selected := selectRows(list, names)
			for j, name := range names {
				if !selected[j] {
					delete(rows, name)
				}
			}
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
					filterMap(result)
				}
			}
		}
		if before > 0 {
			appendNote(&f.Notes, fmt.Sprintf("Kept %d of %d market rows: prepared/requested symbols and representative product, settlement and contract-unit types. Retained rows and original totals are unchanged.", after, before))
		}
	}
}
