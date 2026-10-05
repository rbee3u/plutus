package valuation

import (
	"bytes"
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/ordering"
)

func sortAccounts(accounts []AccountValuation) {
	key := func(a AccountValuation) ordering.Account {
		return ordering.Account{
			Identity:  ordering.Identity{Platform: a.Platform, Name: a.IdentityName, ID: a.IdentityID},
			Structure: adapters.AccountStructure(a.accountStructure), Type: adapters.AccountType(a.AccountType), ID: a.AccountID,
		}
	}
	slices.SortFunc(accounts, func(a, b AccountValuation) int {
		ka, kb := key(a), key(b)
		return ordering.CompareAccounts(&ka, &kb)
	})
}

// accountRanks preserves the hierarchy already established by Accounts for all
// flat holdings reads. Rows are sorted before totals are grouped or pages sliced.
func accountRanks(accounts []AccountValuation) map[uuid.UUID]int {
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
		if c := strings.Compare(a.Symbol, b.Symbol); c != 0 {
			return c
		}
		return bytes.Compare(a.ListingID[:], b.ListingID[:])
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
		if row.BaseSymbol == nil {
			return ""
		}
		return strings.ToUpper(strings.TrimSpace(*row.BaseSymbol))
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
	// Missing Base metadata falls back to the native Symbol without guessing an Asset.
	if ab != "" {
		if c := cmp.Compare(positionKindRank(a.Kind), positionKindRank(b.Kind)); c != 0 {
			return c
		}
		if a.Kind == string(adapters.KindLinearDeliv) || a.Kind == string(adapters.KindInverseDlv) {
			if c := compareExpiry(a.ExpiryAt, b.ExpiryAt); c != 0 {
				return c
			}
		}
	}
	if c := strings.Compare(a.Symbol, b.Symbol); c != 0 {
		return c
	}
	if c := strings.Compare(a.Side, b.Side); c != 0 {
		return c
	}
	return strings.Compare(a.PlatformPositionRef, b.PlatformPositionRef)
}

func positionKindRank(kind string) int {
	switch kind {
	case adapters.KindLinearPerp:
		return 0
	case adapters.KindLinearDeliv:
		return 1
	case adapters.KindInversePerp:
		return 2
	case adapters.KindInverseDlv:
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
		return strings.Compare(a.PlatformPositionRef, b.PlatformPositionRef)
	})
}
