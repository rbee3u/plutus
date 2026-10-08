package financial

import (
	"strings"

	"github.com/rbee3u/plutus/internal/application/valuation"
)

func matchesAccount(f *ListFilter, a *valuation.AccountValuation) bool {
	if f.Platform != "" && f.Platform != a.Platform {
		return false
	}
	if f.IdentityID != nil && *f.IdentityID != a.IdentityID {
		return false
	}
	return true
}

// matchesSymbol applies a literal, case-insensitive substring search.
func matchesSymbol(query string, symbols ...string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, symbol := range symbols {
		if strings.Contains(strings.ToLower(symbol), query) {
			return true
		}
	}
	return false
}

func optionalSymbol(symbol *string) string {
	if symbol == nil {
		return ""
	}
	return *symbol
}
