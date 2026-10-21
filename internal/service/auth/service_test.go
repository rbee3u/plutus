package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestNewPortfolioIntervals(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := context.Background()
	user, err := NewService(db, time.Hour, 24*time.Hour).CreateUser(ctx, "interval-"+uuid.NewString()[:8], "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios := portfolio.NewService(db)
	initial, err := portfolios.List(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(initial) != 1 || initial[0].CollectionIntervalSeconds != 1800 {
		t.Fatalf("User's first portfolio must start at 30 minutes: %+v", initial)
	}
	secondary, err := portfolios.Create(ctx, user.ID, portfolio.CreateInput{Name: "secondary", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if secondary.CollectionIntervalSeconds != 1800 {
		t.Fatalf("subsequent portfolio must also start at 30 minutes: %+v", secondary)
	}
	interval := 1800
	custom, err := portfolios.Create(ctx, user.ID, portfolio.CreateInput{
		Name: "custom", Timezone: "UTC", CollectionIntervalSeconds: &interval,
	})
	if err != nil {
		t.Fatal(err)
	}
	if custom.CollectionIntervalSeconds != interval {
		t.Fatal("each portfolio must retain its own selectable interval")
	}
}

func TestSessionReadQueriesAndExpiryPrecision(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	service := NewService(db, time.Hour, 24*time.Hour)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	user, err := service.CreateUser(t.Context(), "session-"+uuid.NewString()[:8], "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	_, session, err := service.Login(t.Context(), user.Username, "Correct=123456", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), "UPDATE sessions SET idle_expires_at=?,absolute_expires_at=? WHERE id=?", now, now, session.ID[:]); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []time.Duration{0, time.Nanosecond} {
		now = now.Add(offset)
		want := 1
		if offset > 0 {
			want = 0
		}
		stats.Queries.Store(0)
		items, err := service.ListSessions(t.Context(), user.ID, session.ID)
		if err != nil || len(items) != want {
			t.Fatalf("list at %s: %+v %v", now, items, err)
		}
		count, err := service.CountSessions(t.Context(), user.ID)
		if err != nil || count != want {
			t.Fatalf("count at %s: %d %v", now, count, err)
		}
		if queries := stats.Queries.Load(); queries != 2 {
			t.Fatalf("list/count should each use one read: %d", queries)
		}
	}
	if _, _, err := service.Authenticate(t.Context(), session.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired fraction accepted: %v", err)
	}
	now = now.Add(-time.Nanosecond)
	stats.Queries.Store(0)
	_, loaded, err := service.Authenticate(t.Context(), session.Token)
	if err != nil || !loaded.LastSeenAt.Equal(now) || !loaded.IdleExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("authentication or touch changed: %+v %v", loaded, err)
	}
	if queries := stats.Queries.Load(); queries != 1 {
		t.Fatalf("authentication should join session and user: %d reads", queries)
	}
}

func TestCreateLoginChangePasswordAndDisable(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	svc := NewService(db, time.Hour, 24*time.Hour)
	ctx := context.Background()
	username := "auth-" + uuid.NewString()[:8]
	password := "Correct=123456"

	user, err := svc.CreateUser(ctx, username, password)
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != username {
		t.Fatalf("username %q", user.Username)
	}

	if _, _, err := svc.Login(ctx, username, "wrong password!!", "127.0.0.1", "test"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}

	loggedIn, session, err := svc.Login(ctx, username, password, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if loggedIn.ID != user.ID || session.Token == "" || session.CSRF == "" {
		t.Fatalf("session %+v user %+v", session, loggedIn)
	}

	authed, loaded, err := svc.Authenticate(ctx, session.Token)
	if err != nil || authed.ID != user.ID || loaded.ID != session.ID {
		t.Fatalf("authenticate: user=%+v session=%+v err=%v", authed, loaded, err)
	}

	if err := svc.ChangePassword(ctx, user.ID, password, password); !errors.Is(err, ErrSamePassword) {
		t.Fatalf("expected ErrSamePassword, got: %v", err)
	}

	newPassword := "NewPass=123456"
	_, secondSession, err := svc.Login(ctx, username, password, "127.0.0.2", "second-agent")
	if err != nil {
		t.Fatal(err)
	}
	other, err := svc.CreateUser(ctx, username+"-other", password)
	if err != nil {
		t.Fatal(err)
	}
	_, otherSession, err := svc.Login(ctx, other.Username, password, "127.0.0.3", "other-agent")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangePassword(ctx, user.ID, password, newPassword); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(ctx, username, password, "10.0.0.2", "old"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password still worked: %v", err)
	}

	// Password change revokes all sessions, including the current one.
	for name, token := range map[string]string{"current": session.Token, "second": secondSession.Token} {
		if _, _, err := svc.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("%s session should be revoked, got: %v", name, err)
		}
	}
	if got, _, err := svc.Authenticate(ctx, otherSession.Token); err != nil || got.ID != other.ID {
		t.Fatalf("another user's session changed: user=%+v err=%v", got, err)
	}

	_, newSession, err := svc.Login(ctx, username, newPassword, "127.0.0.1", "new-agent")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.DisableUser(ctx, username); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Authenticate(ctx, newSession.Token); !errors.Is(err, ErrUserDisabled) && !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("disabled session: %v", err)
	}
}

func TestPasswordResetSerializesWithLoginAndUTF8SessionLabels(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	for range 3 {
		svc := NewService(db, time.Hour, 24*time.Hour)
		name := "race-" + uuid.NewString()[:8]
		if _, err := svc.CreateUser(ctx, name, "Correct=123456"); err != nil {
			t.Fatal(err)
		}
		type outcome struct {
			session Session
			err     error
		}
		login := make(chan outcome, 1)
		reset := make(chan error, 1)
		start := make(chan struct{})
		go func() {
			<-start
			_, session, err := svc.Login(ctx, name, "Correct=123456", "127.0.0.1", strings.Repeat("币", 260)+"\xff")
			login <- outcome{session, err}
		}()
		go func() { <-start; reset <- svc.ResetPassword(ctx, name, "Replacement=123456") }()
		close(start)
		result := <-login
		if err := <-reset; err != nil {
			t.Fatal(err)
		}
		if result.err != nil && !errors.Is(result.err, ErrInvalidCredentials) {
			t.Fatal(result.err)
		}
		if result.err == nil {
			if result.session.UserAgent == nil || !utf8.ValidString(*result.session.UserAgent) || utf8.RuneCountInString(*result.session.UserAgent) != 255 {
				t.Fatalf("invalid session label: %+v", result.session.UserAgent)
			}
			if _, _, err := svc.Authenticate(ctx, result.session.Token); err == nil {
				t.Fatal("old-password login survived reset")
			}
		}
		if _, _, err := svc.Login(ctx, name, "Replacement=123456", "127.0.0.1", "new password"); err != nil {
			t.Fatalf("password replacement overwritten: %v", err)
		}
	}
}

func TestPasswordAndRevocationRollbackTogether(t *testing.T) {
	db := mysqltest.OpenTestDB(t)
	ctx := t.Context()
	svc := NewService(db, time.Hour, 24*time.Hour)
	name := "atomic-" + uuid.NewString()[:8]
	user, err := svc.CreateUser(ctx, name, "Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	_, session, err := svc.Login(ctx, name, "Correct=123456", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	trigger := "revoke_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.ExecContext(ctx, "CREATE TRIGGER "+trigger+" BEFORE UPDATE ON sessions FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test revocation failure'"); err != nil {
		t.Fatal(err)
	} // #nosec G202 -- generated internal identifier.
	defer func() { _, _ = db.ExecContext(ctx, "DROP TRIGGER IF EXISTS "+trigger) }() // #nosec G202 -- generated internal identifier.
	if err := svc.ResetPassword(ctx, name, "Replacement=123456"); err == nil {
		t.Fatal("injected failure ignored")
	}
	row, err := svc.q.GetUserByID(ctx, user.ID[:])
	if err != nil {
		t.Fatal(err)
	}
	valid, _, err := VerifyPassword("Correct=123456", row.PasswordHash)
	if err != nil || !valid {
		t.Fatal("password committed despite failed revocation")
	}
	if _, err := db.ExecContext(ctx, "DROP TRIGGER "+trigger); err != nil {
		t.Fatal(err)
	} // #nosec G202 -- generated internal identifier.
	if _, _, err := svc.Authenticate(ctx, session.Token); err != nil {
		t.Fatalf("session changed after rollback: %v", err)
	}
}
