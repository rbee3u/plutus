package seeds

import (
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/auth"
)

const (
	alicePasswordHash = `$argon2id$v=19$m=65536,t=3,p=2$4ZKp/JFnZmxIWjXk18zPWA$3wK/YAxLVIGX9PZpBe9MxZGrZpC+pw3nVpWfK6q2s2I`
	bobPasswordHash   = `$argon2id$v=19$m=65536,t=3,p=2$2Iobo6Ik3eLKnVXVTJvdKg$6yClSIX/cdFRxSZXqB1LUHEDpU0ah/bdDgC+pIu4EW8`
)

func TestMinimalSQLHasExpectedFacts(t *testing.T) {
	t.Parallel()

	sql, err := MinimalSQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "alice") || !strings.Contains(sql, "'bob'") {
		t.Fatal("minimal seed must contain both users")
	}
	if !strings.Contains(sql, "'default'") || !strings.Contains(sql, "'trading'") {
		t.Fatal("minimal seed must contain two portfolios for alice")
	}
	assertNoProjectionRows(t, sql)
	assertBalancedParentheses(t, sql)
}

func TestEdgeCasesSQLHasExpectedFacts(t *testing.T) {
	t.Parallel()

	sql, err := EdgeCasesSQL()
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		"alice",
		"'bob'",
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
	assertBalancedParentheses(t, sql)
}

func TestWalkthroughSQLHasExpectedFacts(t *testing.T) {
	t.Parallel()

	sql, err := WalkthroughSQL()
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		"alice",
		"'bob'",
		"'trading'",
		"TOKENX",
		"provisional",
		"collection_failures",
		"'partial'",
		"pos-alice-btc-short",
		"'short'",
		"edited contribution",
		"2026-08-31 16:00:00.000000",
		"UniMMR",
		"Alice Binance Sub",
		"Alice OKX Main",
		"BNB",
		"XRP",
		"DOGE",
		"2026-09-20 16:00:00.000000",
		"WBTC",
		"catalog_pins",
		"account_metric_observations",
		"observation.correct",
		"'invalid'",
		"'disappeared'",
		"pos-alice-okx-btc",
	} {
		if !strings.Contains(sql, needle) {
			t.Fatalf("walkthrough seed must contain %q", needle)
		}
	}
	if strings.Count(sql, "INSERT INTO observations") != 1 {
		t.Fatal("walkthrough should insert observations in one statement")
	}
	assertNoProjectionRows(t, sql)
	assertBalancedParentheses(t, sql)
}

func TestSeedIDMatchesCanonical(t *testing.T) {
	t.Parallel()
	if UserAlice.String() != "01a0bdfc-2b00-7001-8000-000000000001" {
		t.Fatalf("alice id %s", UserAlice)
	}
	if ListingBinanceUSDT.String() != "01a0bdfc-2b00-7007-8000-000000000001" {
		t.Fatalf("listing id %s", ListingBinanceUSDT)
	}
}

func TestSeedPasswordHashes(t *testing.T) {
	t.Parallel()

	minimal, err := MinimalSQL()
	if err != nil {
		t.Fatal(err)
	}
	edge, err := EdgeCasesSQL()
	if err != nil {
		t.Fatal(err)
	}
	walkthrough, err := WalkthroughSQL()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, password, hash string
	}{
		{"alice", "Alice=123456", alicePasswordHash},
		{"bob", "Bob=123456", bobPasswordHash},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, _, err := auth.VerifyPassword(tc.password, tc.hash)
			if err != nil || !ok {
				t.Fatalf("verify: ok=%v err=%v", ok, err)
			}
			if !strings.Contains(minimal, tc.hash) || !strings.Contains(edge, tc.hash) || !strings.Contains(walkthrough, tc.hash) {
				t.Fatal("every seed profile must contain the hash")
			}
		})
	}
}

func assertNoProjectionRows(t *testing.T, sql string) {
	t.Helper()
	if strings.Contains(sql, "INSERT INTO latest_balance_projections") ||
		strings.Contains(sql, "INSERT INTO projection_generations") {
		t.Fatal("seeds must not insert projection rows")
	}
}

func assertBalancedParentheses(t *testing.T, sql string) {
	t.Helper()
	depth := 0
	for i, r := range sql {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				t.Fatalf("extra closing parenthesis at byte %d", i)
			}
		}
	}
	if depth != 0 {
		t.Fatalf("unbalanced parentheses: %d still open", depth)
	}
}
