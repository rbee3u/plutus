package valuation

import (
	"slices"

	"github.com/rbee3u/plutus/internal/application/ordering"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func sortAccounts(accounts []AccountValuation) {
	key := func(a AccountValuation) ordering.Account {
		return ordering.Account{
			Identity:  ordering.Identity{Platform: a.Platform, Name: a.IdentityName, ID: a.IdentityID},
			Structure: adapters.AccountStructure(a.AccountStructure), Type: adapters.AccountType(a.AccountType), ID: a.AccountID,
		}
	}
	slices.SortFunc(accounts, func(a, b AccountValuation) int {
		return ordering.CompareAccounts(key(a), key(b))
	})
}
