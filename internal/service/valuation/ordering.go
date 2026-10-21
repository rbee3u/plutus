package valuation

import (
	"slices"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/ordering"
)

func sortAccounts(accounts []AccountValuation) {
	key := func(a AccountValuation) ordering.Account {
		return ordering.Account{
			Identity:  ordering.Identity{Platform: a.Platform, Name: a.IdentityName, ID: a.IdentityID},
			Structure: adapter.AccountStructure(a.AccountStructure), Type: adapter.AccountType(a.AccountType), ID: a.AccountID,
		}
	}
	slices.SortFunc(accounts, func(a, b AccountValuation) int {
		return ordering.CompareAccounts(key(a), key(b))
	})
}
