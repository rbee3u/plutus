package financial

import (
	"strings"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/service/valuation"
)

func reportCoverage(accounts []valuation.AccountValuation, f *ListFilter, kind string, hasPoint bool) ReportCoverage {
	out := ReportCoverage{Complete: hasPoint, MissingAccountIDs: []uuid.UUID{}}
	for i := range accounts {
		a := &accounts[i]
		if !matchesHoldingAccount(f, a, kind) {
			continue
		}
		if a.SourceReportID == nil {
			out.Complete = false
			out.MissingAccountIDs = append(out.MissingAccountIDs, a.AccountID)
		}
	}
	return out
}

func matchesHoldingAccount(f *ListFilter, a *valuation.AccountValuation, kind string) bool {
	if !matchesAccount(f, a) {
		return false
	}
	if kind == "positions" {
		return positionAccount(a)
	}
	return kind == "" || a.AccountType == kind
}

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

func optionalFilterID(id *uuid.UUID) []byte {
	if id == nil {
		return nil
	}
	return id[:]
}

func normalizedSearch(q string) string { return strings.ToLower(strings.TrimSpace(q)) }
