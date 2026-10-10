package manual

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Historical revisions and frozen cycle inputs are immutable. Only this storage
// boundary accepts their old keys; the public API writes and accepts new names.
func decodeBalances(raw []byte) ([]Balance, error) {
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, errors.New("manual history: balances must be an array")
	}
	balances := make([]Balance, 0, len(rows))
	for _, row := range rows {
		for _, keys := range [][2]string{{"listingId", "platformAssetId"}, {"symbol", "platformAssetSymbol"}} {
			legacy, exists := row[keys[0]]
			if !exists {
				continue
			}
			if current, exists := row[keys[1]]; exists {
				var a, b string
				if json.Unmarshal(legacy, &a) != nil || json.Unmarshal(current, &b) != nil || a != b {
					return nil, fmt.Errorf("manual history: conflicting %s and %s", keys[0], keys[1])
				}
			} else {
				row[keys[1]] = legacy
			}
			delete(row, keys[0])
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		var balance Balance
		if err := json.Unmarshal(encoded, &balance); err != nil {
			return nil, err
		}
		if balance.AssetID == uuid.Nil || balance.AssetSymbol == "" || balance.CanonicalAssetSymbol == "" {
			return nil, errors.New("manual history: missing asset identity")
		}
		balances = append(balances, balance)
	}
	return balances, nil
}
