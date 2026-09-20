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
	if strings.Contains(sql, "INSERT INTO latest_balance_projections") {
		t.Fatal("seeds must not insert projection rows")
	}
}
