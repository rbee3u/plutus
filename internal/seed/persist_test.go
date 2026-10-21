package seed

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestApplyRollsBackSourceInsertionFailure(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := context.Background()
	id := uuid.New()
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id=?", id[:]); err != nil {
			t.Error(err)
		}
	})
	b := newBuilder()
	b.user(userSpec{ID: id, Username: "rollback-" + uuid.NewString()[:8], Hash: alicePasswordHash, CreatedAt: time.Now()})
	// The second statement violates the unique user ID. The first must roll back.
	source := b.SQL()
	if err := Apply(ctx, db, Seed{SQL: source + source}); err == nil {
		t.Fatal("duplicate source insertion was accepted")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE id=?", id[:]).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial seed remained: count=%d error=%v", count, err)
	}
	if err := Apply(ctx, db, Seed{SQL: source}); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

func TestApplyRollsBackInitializedPointsOnFailure(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := context.Background()
	userID, portfolioID, missingPortfolioID := uuid.New(), uuid.New(), uuid.New()
	at := time.Now().UTC().Truncate(time.Microsecond)
	b := newBuilder()
	b.user(userSpec{ID: userID, Username: "point-rollback-" + uuid.NewString()[:8], Hash: alicePasswordHash, CreatedAt: at})
	b.portfolio(&portfolioSpec{ID: portfolioID, User: userID, Name: "rollback", Timezone: "UTC", Interval: 900, CreatedAt: at})
	b.cycle(portfolioID, at, nil)
	seed := Seed{SQL: b.SQL(), Portfolios: []uuid.UUID{portfolioID, missingPortfolioID}}
	if b.err != nil {
		t.Fatal(b.err)
	}
	// The first portfolio's point is initialized before the missing second
	// portfolio fails. Both its source rows and derived point must roll back.
	if err := Apply(ctx, db, seed); !errors.Is(err, sql.ErrNoRows) || !strings.Contains(err.Error(), missingPortfolioID.String()) {
		t.Fatalf("expected missing initialization portfolio: %v", err)
	}
	for _, check := range []struct {
		query string
		id    uuid.UUID
	}{
		{"SELECT COUNT(*) FROM users WHERE id=?", userID},
		{"SELECT COUNT(*) FROM collection_cycles WHERE portfolio_id=?", portfolioID},
		{"SELECT COUNT(*) FROM valuation_points WHERE portfolio_id=?", portfolioID},
	} {
		var count int
		if err := db.QueryRowContext(ctx, check.query, check.id[:]).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial seed remained: query=%s count=%d error=%v", check.query, count, err)
		}
	}
}
