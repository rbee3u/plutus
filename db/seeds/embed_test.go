package seeds

import (
	"strings"
	"testing"
)

func TestMinimalSQLHasExpectedFacts(t *testing.T) {
	t.Parallel()

	sql, err := MinimalSQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "alice") || !strings.Contains(sql, "bob") {
		t.Fatal("minimal seed must contain both users")
	}
	if !strings.Contains(sql, "'Main'") || !strings.Contains(sql, "'Trading'") {
		t.Fatal("minimal seed must contain two portfolios for alice")
	}
	assertNoProjectionRows(t, sql)
}

func TestEdgeCasesSQLHasExpectedFacts(t *testing.T) {
	t.Parallel()

	sql, err := EdgeCasesSQL()
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		"alice",
		"bob",
		"TOKENX",
		"provisional",
		"collection_failures",
		"'partial'",
		"pos-alice-btc-short",
		"'short'",
		"edited contribution",
		"2026-08-31 16:00:00.000000",
	} {
		if !strings.Contains(sql, needle) {
			t.Fatalf("edge-cases seed must contain %q", needle)
		}
	}
	assertNoProjectionRows(t, sql)
}

func assertNoProjectionRows(t *testing.T, sql string) {
	t.Helper()
	if strings.Contains(sql, "INSERT INTO latest_balance_projections") ||
		strings.Contains(sql, "INSERT INTO projection_generations") {
		t.Fatal("seeds must not insert projection rows")
	}
}
