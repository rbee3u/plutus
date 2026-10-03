package recording

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

func TestExchangeInfoKeepsWhitelistedContractsBeyondFirstType(t *testing.T) {
	s := New(adapters.PlatformBinance, fixture.Request{Service: "cm", Method: "GET", Path: "/dapi/v1/exchangeInfo"})
	f, err := s.Run(context.Background(), adapters.Credentials{}, func(context.Context) error {
		s.Add(s.request, 200, []byte(`{"symbols":[
			{"symbol":"BTCUSD_PERP","baseAsset":"BTC","quoteAsset":"USD","contractType":"PERPETUAL","contractSize":100},
			{"symbol":"ETHUSD_PERP","baseAsset":"ETH","quoteAsset":"USD","contractType":"PERPETUAL","contractSize":10},
			{"symbol":"BTCUSD_261225","baseAsset":"BTC","quoteAsset":"USD","contractType":"CURRENT_QUARTER","contractSize":100,"deliveryDate":1798185600000},
			{"symbol":"ETHUSD_261225","baseAsset":"ETH","quoteAsset":"USD","contractType":"CURRENT_QUARTER","contractSize":10,"deliveryDate":1798185600000},
			{"symbol":"DOGEUSD_PERP","baseAsset":"DOGE","quoteAsset":"USD","contractType":"PERPETUAL","contractSize":10}
		]}`))
		return nil
	})
	if err != nil || f == nil {
		t.Fatalf("capture: file=%+v err=%v", f, err)
	}
	var body struct {
		Symbols []struct {
			Symbol       string `json:"symbol"`
			ContractSize int    `json:"contractSize"`
			DeliveryDate int64  `json:"deliveryDate"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(f.Body, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Symbols) != 4 || !strings.Contains(f.Notes, "Kept 4 of 5 market rows") {
		t.Fatalf("whitelist must keep BTC/ETH contracts and trim unrelated rows: %s; %s", f.Body, f.Notes)
	}
	seen := map[string]bool{}
	for _, row := range body.Symbols {
		seen[row.Symbol] = true
		if strings.HasPrefix(row.Symbol, "ETHUSD") && row.ContractSize != 10 {
			t.Fatalf("ETH contract size changed: %+v", row)
		}
		if strings.HasSuffix(row.Symbol, "_261225") && row.DeliveryDate != 1798185600000 {
			t.Fatalf("delivery date changed: %+v", row)
		}
	}
	for _, symbol := range []string{"BTCUSD_PERP", "ETHUSD_PERP", "BTCUSD_261225", "ETHUSD_261225"} {
		if !seen[symbol] {
			t.Fatalf("missing whitelisted contract %s", symbol)
		}
	}
}

func TestMarketSelectionPreservesPreparedProductsAndNewShapes(t *testing.T) {
	s := New(adapters.PlatformBinance, fixture.Request{Service: "um", Method: "GET", Path: "/fapi/v1/exchangeInfo"}, Options{KeepSymbols: []string{"DOGEUSDT"}})
	f, err := s.Run(context.Background(), adapters.Credentials{}, func(context.Context) error {
		s.Add(s.request, 200, []byte(`{"total":5,"symbols":[
		{"symbol":"AAAUSDT","contractType":"PERPETUAL","baseAsset":"AAA","quoteAsset":"USDT","marginAsset":"USDT"},
		{"symbol":"BTCUSDT","contractType":"PERPETUAL","baseAsset":"BTC","quoteAsset":"USDT","marginAsset":"USDT"},
		{"symbol":"DOGEUSDT","contractType":"PERPETUAL","baseAsset":"DOGE","quoteAsset":"USDT","marginAsset":"USDT"},
		{"symbol":"NEWUSDT","contractType":"PERPETUAL","baseAsset":"NEW","quoteAsset":"USDT","marginAsset":"USDT","newField":null},
		{"symbol":"BTCUSDT_QUARTER","contractType":"CURRENT_QUARTER","baseAsset":"BTC","quoteAsset":"USDT","marginAsset":"USDT"}]}`))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Total   int              `json:"total"`
		Symbols []map[string]any `json:"symbols"`
	}
	if err := json.Unmarshal(f.Body, &body); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range body.Symbols {
		seen[row["symbol"].(string)] = true
	}
	if body.Total != 5 || len(seen) != 4 || seen["AAAUSDT"] || !seen["BTCUSDT"] || !seen["DOGEUSDT"] || !seen["NEWUSDT"] || !seen["BTCUSDT_QUARTER"] {
		t.Fatalf("representatives/new shape: %s", f.Body)
	}
	if len(f.Request.Query) != 0 || strings.Contains(string(f.Body), "keepSymbols") {
		t.Fatal("local selection changed request or response fields")
	}
}

func TestZeroRowsKeepLiabilitiesSettingsTypesAndUnknownValues(t *testing.T) {
	s := New(adapters.PlatformBinance, fixture.Request{Service: "portfolio", Method: "GET", Path: "/papi/v1/balance"})
	f, err := s.Run(context.Background(), adapters.Credentials{}, func(context.Context) error {
		s.Add(s.request, 200, []byte(`[
		{"asset":"BTC","totalWalletBalance":"0","crossMarginBorrowed":"0","crossMarginInterest":"0"},
		{"asset":"ETH","totalWalletBalance":"0","crossMarginBorrowed":"0","crossMarginInterest":"0"},
		{"asset":"USDT","totalWalletBalance":"0","crossMarginBorrowed":"2","crossMarginInterest":"0.01"},
		{"asset":"NULL","totalWalletBalance":null,"crossMarginBorrowed":"0","crossMarginInterest":"0"},
		{"asset":"BLANK","totalWalletBalance":"","crossMarginBorrowed":"0","crossMarginInterest":"0"},
		{"asset":"META_BLANK_A","totalWalletBalance":"0","marginType":""},
		{"asset":"META_BLANK_B","totalWalletBalance":"0","marginType":""},
		{"asset":"NEW","totalWalletBalance":"0","newMeasure":"0"},
		{"asset":"NUMBER","totalWalletBalance":0,"crossMarginBorrowed":"0","crossMarginInterest":"0"}
		]`))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(f.Body, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 || !strings.Contains(f.Notes, "Kept 8 of 9") {
		t.Fatalf("unsafe zero pruning: %s; %s", f.Body, f.Notes)
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row["asset"].(string)] = true
	}
	for _, asset := range []string{"BTC", "USDT", "NULL", "BLANK", "META_BLANK_A", "META_BLANK_B", "NEW", "NUMBER"} {
		if !seen[asset] {
			t.Fatalf("lost %s", asset)
		}
	}
	if seen["ETH"] {
		t.Fatal("redundant zero row retained")
	}
}

func TestZeroPositionRowsKeepModesAndNonzeroIsolatedMargin(t *testing.T) {
	s := New(adapters.PlatformBinance, fixture.Request{Service: "cm", Method: "GET", Path: "/dapi/v1/positionRisk"})
	f, err := s.Run(context.Background(), adapters.Credentials{}, func(context.Context) error {
		s.Add(s.request, 200, []byte(`[
		{"symbol":"BTCUSD_PERP","positionAmt":"0","positionSide":"LONG","marginType":"cross","isolatedWallet":"0"},
		{"symbol":"ETHUSD_PERP","positionAmt":"0","positionSide":"LONG","marginType":"cross","isolatedWallet":"0"},
		{"symbol":"BTCUSD_PERP","positionAmt":"0","positionSide":"SHORT","marginType":"cross","isolatedWallet":"0"},
		{"symbol":"ETHUSD_PERP","positionAmt":"0","positionSide":"BOTH","marginType":"isolated","isolatedWallet":"0.01"},
		{"symbol":"SOLUSD_PERP","positionAmt":"-1","positionSide":"BOTH","marginType":"cross","isolatedWallet":"0"}
		]`))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(f.Body, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || !strings.Contains(f.Notes, "Kept 4 of 5") {
		t.Fatalf("position pruning %s; %s", f.Body, f.Notes)
	}
}

func TestZeroNestedBalancesKeepOriginalTotalsAndProductLists(t *testing.T) {
	for _, tc := range []struct {
		path, raw string
		want      int
	}{
		{"/v5/account/wallet-balance", `{"result":{"list":[{"totalEquity":"123","coin":[{"coin":"BTC","walletBalance":"0"},{"coin":"ETH","walletBalance":"0"},{"coin":"USDT","walletBalance":"5"}]}]}}`, 2},
		{"/api/v5/account/balance", `{"code":"0","data":[{"totalEq":"123","details":[{"ccy":"BTC","cashBal":"0"},{"ccy":"ETH","cashBal":"0"},{"ccy":"USDT","cashBal":"5"}]}]}`, 2},
		{"/sapi/v1/simple-earn/flexible/position", `{"total":3,"rows":[{"asset":"BTC","free":"0"},{"asset":"ETH","free":"0"},{"asset":"USDT","free":"5"}]}`, 3},
	} {
		t.Run(tc.path, func(t *testing.T) {
			s := New(adapters.PlatformBybit, fixture.Request{Service: "v5", Method: "GET", Path: tc.path})
			f, err := s.Run(context.Background(), adapters.Credentials{}, func(context.Context) error { s.Add(s.request, 200, []byte(tc.raw)); return nil })
			if err != nil {
				t.Fatal(err)
			}
			if count := strings.Count(string(f.Body), `"BTC"`) + strings.Count(string(f.Body), `"ETH"`) + strings.Count(string(f.Body), `"USDT"`); count != tc.want {
				t.Fatalf("rows=%d: %s", count, f.Body)
			}
			if strings.Contains(tc.raw, "123") && !strings.Contains(string(f.Body), `"123"`) {
				t.Fatal("original aggregate rewritten")
			}
		})
	}
}
