package financial

import "testing"

func TestSymbolSubstringSearch(t *testing.T) {
	for _, test := range []struct {
		name, query string
		symbols     []string
		want        bool
	}{
		{"native contract", "btc", []string{"BTC-USDT-SWAP"}, true},
		{"mapped asset", "bTc", []string{"XBT", "BTC"}, true},
		{"quote currency", "sDt", []string{"BTC", "USDT"}, true},
		{"non-ASCII", "牛来", []string{"牛来USDT"}, true},
		{"empty optional terms", "BTC", []string{"USDT", "", ""}, false},
		{"literal wildcard", "BTC%", []string{"BTCUSDT"}, false},
		{"literal underscore", "BTC_", []string{"BTC-USD"}, false},
		{"surrounding whitespace", "  btc ", []string{"BTCUSDT"}, true},
		{"empty query", "  ", nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := matchesSymbol(test.query, test.symbols...); got != test.want {
				t.Fatalf("matchesSymbol(%q, %v) = %v, want %v", test.query, test.symbols, got, test.want)
			}
		})
	}
}
