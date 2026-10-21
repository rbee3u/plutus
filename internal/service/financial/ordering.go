package financial

import (
	"bytes"
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

// accountRanks preserves the hierarchy already established by accounts for all
// flat holdings reads. Rows are sorted before totals are grouped or pages sliced.
func accountRanks(accounts []valuation.AccountValuation) map[uuid.UUID]int {
	ranks := make(map[uuid.UUID]int, len(accounts))
	for i := range accounts {
		ranks[accounts[i].AccountID] = i
	}
	return ranks
}

func sortBalances(rows []BalanceRow, ranks map[uuid.UUID]int) {
	slices.SortFunc(rows, func(a, b BalanceRow) int {
		if c := cmp.Compare(ranks[a.AccountID], ranks[b.AccountID]); c != 0 {
			return c
		}
		if c := strings.Compare(a.AssetSymbol, b.AssetSymbol); c != 0 {
			return c
		}
		if c := strings.Compare(a.CanonicalAssetSymbol, b.CanonicalAssetSymbol); c != 0 {
			return c
		}
		return bytes.Compare(a.AssetID[:], b.AssetID[:])
	})
}

func sortPositions(rows []PositionRow, ranks map[uuid.UUID]int) {
	slices.SortFunc(rows, func(a, b PositionRow) int {
		if c := cmp.Compare(ranks[a.AccountID], ranks[b.AccountID]); c != 0 {
			return c
		}
		return comparePositions(&a, &b)
	})
}

func comparePositions(a, b *PositionRow) int {
	base := func(row *PositionRow) string {
		if row.BaseAssetSymbol == nil {
			return ""
		}
		return strings.ToUpper(strings.TrimSpace(*row.BaseAssetSymbol))
	}
	ab, bb := base(a), base(b)
	if ab == "" && bb != "" {
		return 1
	}
	if ab != "" && bb == "" {
		return -1
	}
	if c := strings.Compare(ab, bb); c != 0 {
		return c
	}
	// Missing base metadata falls back to the native symbol without guessing an asset.
	if ab != "" {
		if c := cmp.Compare(positionKindRank(a.Kind), positionKindRank(b.Kind)); c != 0 {
			return c
		}
		if a.Kind == string(adapter.KindLinearDelivery) || a.Kind == string(adapter.KindInverseDelivery) {
			if c := compareExpiry(a.ExpiryAt, b.ExpiryAt); c != 0 {
				return c
			}
		}
	}
	if c := strings.Compare(a.FuturesSymbol, b.FuturesSymbol); c != 0 {
		return c
	}
	if c := strings.Compare(a.Side, b.Side); c != 0 {
		return c
	}
	return strings.Compare(a.PositionKey, b.PositionKey)
}

func positionKindRank(kind string) int {
	switch kind {
	case adapter.KindLinearPerpetual:
		return 0
	case adapter.KindLinearDelivery:
		return 1
	case adapter.KindInversePerpetual:
		return 2
	case adapter.KindInverseDelivery:
		return 3
	default:
		return 4
	}
}

func compareExpiry(a, b *time.Time) int {
	if a == nil {
		if b == nil {
			return 0
		}
		return 1
	}
	if b == nil {
		return -1
	}
	return a.Compare(*b)
}

func sortDualPositions(rows []DualPositionRow, ranks map[uuid.UUID]int) {
	slices.SortFunc(rows, func(a, b DualPositionRow) int {
		if c := cmp.Compare(ranks[a.AccountID], ranks[b.AccountID]); c != 0 {
			return c
		}
		if c := compareExpiry(a.ExpiryAt, b.ExpiryAt); c != 0 {
			return c
		}
		return strings.Compare(a.PositionKey, b.PositionKey)
	})
}

func earnAssetKey(a adapter.EarnAmount) string {
	if a.CanonicalAssetSymbol != nil {
		return a.AssetSymbol + "\x00" + *a.CanonicalAssetSymbol
	}
	return a.AssetSymbol + "\x00" + a.AssetSymbol
}

func compareEarnAssets(a, b adapter.EarnAmount) int {
	return strings.Compare(earnAssetKey(a), earnAssetKey(b))
}

func sortEarnPositions(rows []EarnPositionRow, ranks map[uuid.UUID]int) {
	types := map[string]int{"flexible": 0, "fixed": 1, "bonded": 2, "timed": 3, "on_chain": 4}
	slices.SortFunc(rows, func(a, b EarnPositionRow) int {
		if c := cmp.Compare(ranks[a.AccountID], ranks[b.AccountID]); c != 0 {
			return c
		}
		if c := slices.CompareFunc(a.Assets, b.Assets, compareEarnAssets); c != 0 {
			return c
		}
		if c := cmp.Compare(types[a.Type], types[b.Type]); c != 0 {
			return c
		}
		if c := compareExpiry(a.MaturityAt, b.MaturityAt); c != 0 {
			return c
		}
		return strings.Compare(a.PositionKey, b.PositionKey)
	})
}
