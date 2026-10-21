package adapter

import (
	"math/big"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestTotalsCombineBalancesAndRejectMissingSymbols(t *testing.T) {
	t.Parallel()
	totals := Totals{}
	for _, add := range []struct {
		symbol string
		amount *big.Rat
	}{{"BTC", big.NewRat(1, 2)}, {"BTC", big.NewRat(1, 2)}, {"ETH", big.NewRat(0, 1)}, {" ", big.NewRat(0, 1)}} {
		if err := totals.Add("test", add.symbol, add.amount); err != nil {
			t.Fatal(err)
		}
	}
	balances := totals.Balances()
	if len(balances) != 1 || balances[0].AssetSymbol != "BTC" || balances[0].Equity != "1" {
		t.Fatalf("Totals.Balances = %+v", balances)
	}
	if class, _ := Classify(totals.Add("test", " ", big.NewRat(5, 1))); class != ClassNormalization {
		t.Fatalf("a nonzero amount without a coin must fail, not vanish: %q", class)
	}
}

func FuzzTotalsConserveBalances(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 129, 0, 127, 1, 128, 2, 0})
	f.Add([]byte{2, 255, 1, 0, 0, 129, 2, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		// Each pair chooses a symbol and signed cents. The bounded integer
		// sums are independent of the production rational aggregation.
		data = data[:min(len(data), 256)]
		symbols := []string{"BTC", "ETH", "USDT"}
		sums := make([]int64, len(symbols))
		for i := 0; i+1 < len(data); i += 2 {
			sums[int(data[i])%len(symbols)] += int64(data[i+1]) - 128
		}
		want := []Balance{}
		for i, cents := range sums {
			if cents != 0 {
				amount := big.NewRat(cents, 100).FloatString(2)
				want = append(want, Balance{AssetSymbol: symbols[i], Equity: strings.TrimRight(strings.TrimRight(amount, "0"), ".")})
			}
		}
		for _, split := range []bool{false, true} {
			for _, reversed := range []bool{false, true} {
				indices := make([]int, len(data)/2)
				for i := range indices {
					indices[i] = 2 * i
				}
				if reversed {
					slices.Reverse(indices)
				}
				totals := Totals{}
				for _, i := range indices {
					symbol := symbols[int(data[i])%len(symbols)]
					parts := int64(1)
					if split {
						parts = 2
					}
					for range parts {
						if err := totals.Add("test", symbol, big.NewRat(int64(data[i+1])-128, 100*parts)); err != nil {
							t.Fatal(err)
						}
					}
				}
				if got := totals.Balances(); !reflect.DeepEqual(got, want) {
					t.Fatalf("split=%v reversed=%v: got %+v, want sorted nonzero sums %+v", split, reversed, got, want)
				}
			}
		}
	})
}
