package financial

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
)

func TestMergeManualBalancesPreservesCollectedAmounts(t *testing.T) {
	account := valuation.AccountValuation{AccountID: uuid.New(), AccountType: "manual", IdentityID: uuid.New(), Platform: "okx"}
	value := "20000"
	collected := []BalanceRow{{Symbol: "BTC", Equity: "1", EquityUSD: &value}, {Symbol: "USDT", Equity: "50"}}
	current := []BalanceRow{{Symbol: "BTC", Equity: "-2", ManualQuantity: new("-2")}, {Symbol: "USD", Equity: "100", ManualQuantity: new("100"), ManualChange: "added"}}
	rows := mergeManualBalances(collected, current, &account)
	if len(rows) != 3 || rows[0].Equity != "1" || *rows[0].EquityUSD != value || rows[0].ManualChange != "updated" || *rows[0].ManualQuantity != "-2" {
		t.Fatalf("updated row lost report values: %+v", rows)
	}
	if rows[1].ManualChange != "deleted" || rows[1].Equity != "50" {
		t.Fatalf("deleted row: %+v", rows[1])
	}
	if rows[2].ManualChange != "added" || rows[2].Equity != "100" || rows[2].EquityUSD != nil || rows[2].ReportObservedAt != nil || rows[2].IdentityID != account.IdentityID {
		t.Fatalf("added row: %+v", rows[2])
	}
}

func TestMergeManualBalancesEquivalentAndTinyQuantities(t *testing.T) {
	account := valuation.AccountValuation{AccountType: "manual"}
	for _, pair := range [][2]string{{"1.0", "1"}, {"-0.0000000000000000000000000000010", "-0.000000000000000000000000000001"}} {
		rows := mergeManualBalances([]BalanceRow{{Symbol: "BTC", Equity: pair[0]}}, []BalanceRow{{Symbol: "BTC", Equity: pair[1], ManualQuantity: &pair[1]}}, &account)
		if rows[0].ManualChange != "" || *rows[0].ManualQuantity != pair[1] {
			t.Fatalf("equivalent quantity: %+v", rows[0])
		}
	}
	if rows := mergeManualBalances(nil, nil, &account); len(rows) != 0 {
		t.Fatal(rows)
	}
}
