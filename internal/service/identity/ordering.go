package identity

import (
	"slices"

	"github.com/rbee3u/plutus/internal/service/ordering"
)

func identityOrder(item *Identity) ordering.Identity {
	return ordering.Identity{Platform: string(item.Platform), Name: item.DisplayName, ID: item.ID}
}

func sortIdentityAccounts(item *Identity, accounts []Account) {
	key := func(account Account) ordering.Account {
		return ordering.Account{Identity: identityOrder(item), Structure: item.AccountStructure, Type: account.AccountType, ID: account.ID}
	}
	slices.SortFunc(accounts, func(a, b Account) int {
		return ordering.CompareAccounts(key(a), key(b))
	})
}
