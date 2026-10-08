package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/portfolio"
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
	if len(initial) != 1 || initial[0].CollectionIntervalSeconds != 900 {
		t.Fatalf("User's first Portfolio must start at 15 minutes: %+v", initial)
	}
	secondary, err := portfolios.Create(ctx, user.ID, portfolio.CreateInput{Name: "secondary", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if secondary.CollectionIntervalSeconds != 900 {
		t.Fatalf("subsequent Portfolio must also start at 15 minutes: %+v", secondary)
	}
	interval := 1800
	custom, err := portfolios.Create(ctx, user.ID, portfolio.CreateInput{
		Name: "custom", Timezone: "UTC", CollectionIntervalSeconds: &interval,
	})
	if err != nil {
		t.Fatal(err)
	}
	if custom.CollectionIntervalSeconds != interval {
		t.Fatal("each Portfolio must retain its own selectable interval")
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
		t.Fatalf("another User's session changed: user=%+v err=%v", got, err)
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
