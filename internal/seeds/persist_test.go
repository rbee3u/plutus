package seeds

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func TestApplyRollsBackSourceInsertionFailure(t *testing.T) {
	db := mysql.OpenTestDB(t)
	ctx := context.Background()
	id := uuid.New()
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id=?", id[:]); err != nil {
			t.Error(err)
		}
	})
	b := newBuilder()
	b.user(userSpec{ID: id, Username: "rollback-" + uuid.NewString()[:8], Hash: alicePasswordHash, CreatedAt: time.Now()})
	// The second statement violates the unique User ID. The first must roll back.
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
