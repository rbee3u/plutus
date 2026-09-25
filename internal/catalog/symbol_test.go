package catalog

import (
	"strings"
	"testing"
)

func TestNormalizeSymbol(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		raw, want string
		ok        bool
	}{
		{" usdt ", "USDT", true},
		{"BTCUSD_PERP", "BTCUSD_PERP", true},
		{"1000PEPE", "1000PEPE", true},
		{"牛来USDT", "牛来USDT", true},
		{"币安人生", "币安人生", true},
		{"", "", false},
		{"_BTC", "", false},
		{"BTC USDT", "", false},
		{"BTC/USDT", "", false},
		{strings.Repeat("A", maxSymbol+1), "", false},
	} {
		got, err := NormalizeSymbol(tc.raw)
		if tc.ok != (err == nil) || got != tc.want {
			t.Fatalf("NormalizeSymbol(%q) = %q, %v; want %q ok=%v", tc.raw, got, err, tc.want, tc.ok)
		}
	}
}
