package mysql

import (
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	RunTestMain(m)
}

func TestIsStaleTestDatabase(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_758_600_000, 0)
	fresh := newTestDatabaseName(now.Add(-30 * time.Minute))
	if isStaleTestDatabase(fresh, now) {
		t.Fatalf("%s must not be stale", fresh)
	}
	old := newTestDatabaseName(now.Add(-2 * time.Hour))
	if !isStaleTestDatabase(old, now) {
		t.Fatalf("%s must be stale", old)
	}
	for _, name := range []string{"plutus_local", "plutus_gotest", "plutus_gotest_abc_def", "plutus_test", "plutus_prod"} {
		if isStaleTestDatabase(name, now) {
			t.Fatalf("%s must never be swept", name)
		}
	}
}

func TestCreateTestDatabaseRejectsSelectedDatabase(t *testing.T) {
	t.Parallel()

	_, err := CreateTestDatabase(t.Context(), "user:pw@tcp(127.0.0.1:3306)/plutus_local")
	if err == nil {
		t.Fatal("a DSN that selects a database must be refused")
	}
}
