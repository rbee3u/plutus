// Package ordering defines the shared Platform, Identity, and Account display order.
package ordering

import (
	"bytes"
	"cmp"
	"strings"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// Identity is the stable display key of one Platform Identity.
type Identity struct {
	Platform string
	Name     string
	ID       uuid.UUID
}

// Account extends an Identity key with its Account Type's business order.
type Account struct {
	Identity
	Structure adapters.AccountStructure
	Type      adapters.AccountType
	ID        uuid.UUID
}

// PlatformsCSV supplies the same business order to paginated SQL reads.
func PlatformsCSV() string {
	platforms := adapters.OfferedPlatforms()
	codes := make([]string, len(platforms))
	for i, platform := range platforms {
		codes[i] = string(platform)
	}
	return strings.Join(codes, ",")
}

// ComparePlatforms orders offered Platforms first, followed by unknown codes.
func ComparePlatforms(a, b string) int {
	platforms := adapters.OfferedPlatforms()
	rank := func(code string) int {
		for i, platform := range platforms {
			if string(platform) == code {
				return i
			}
		}
		return len(platforms)
	}
	if c := cmp.Compare(rank(a), rank(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

// CompareIdentities uses case-insensitive names and immutable IDs for ties.
func CompareIdentities(a, b Identity) int {
	if c := ComparePlatforms(a.Platform, b.Platform); c != 0 {
		return c
	}
	if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
		return c
	}
	return bytes.Compare(a.ID[:], b.ID[:])
}

// CompareAccounts ignores editable Account names and financial measures.
func CompareAccounts(a, b Account) int {
	if c := CompareIdentities(a.Identity, b.Identity); c != 0 {
		return c
	}
	if c := cmp.Compare(accountRank(a.Structure, a.Type), accountRank(b.Structure, b.Type)); c != 0 {
		return c
	}
	if c := strings.Compare(string(a.Type), string(b.Type)); c != 0 {
		return c
	}
	return bytes.Compare(a.ID[:], b.ID[:])
}

func accountRank(structure adapters.AccountStructure, accountType adapters.AccountType) int {
	types := adapters.AccountTypes(structure)
	for i, info := range types {
		if info.ID == accountType {
			return i
		}
	}
	return len(types)
}
