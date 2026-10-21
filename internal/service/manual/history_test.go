package manual

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestQuantityComparisonIgnoresLabelsAndEditTimes(t *testing.T) {
	id := uuid.New()
	original := []Balance{{AssetID: id, Quantity: "-2", AssetSymbol: "BTC", CanonicalAssetSymbol: "BTC", ModifiedAt: time.Now()}}
	revised := []Balance{{AssetID: id, Quantity: "-2.00", AssetSymbol: "BTC", CanonicalAssetSymbol: "ETH", ModifiedAt: time.Now().Add(time.Hour)}}
	if !sameQuantities(original, revised) {
		t.Fatal("mapping or edit time became a quantity adjustment")
	}
	revised[0].Quantity = "-3"
	if sameQuantities(original, revised) {
		t.Fatal("changed signed quantity was ignored")
	}
	if !sameQuantities(nil, []Balance{}) {
		t.Fatal("initial revision is not empty")
	}
}
