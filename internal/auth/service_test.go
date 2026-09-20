package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func TestCreateLoginChangePasswordAndDisable(t *testing.T) {
	db := mysql.OpenTestDB(t)
	svc := NewService(db, false, time.Hour, 24*time.Hour)
	ctx := context.Background()
	username := "auth-" + uuid.NewString()[:8]
	password := "correct horse battery staple"

	user, err := svc.CreateUser(ctx, username, password)
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != username {
		t.Fatalf("username %q", user.Username)
	}

	if _, _, err := svc.Login(ctx, username, "wrong password!!", "127.0.0.1", "test"); err != ErrInvalidCredentials {
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

	if err := svc.ChangePassword(ctx, user.ID, session.ID, password, "new horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(ctx, username, password, "10.0.0.2", "old"); err != ErrInvalidCredentials {
		t.Fatalf("old password still worked: %v", err)
	}

	if err := svc.DisableUser(ctx, username); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Authenticate(ctx, session.Token); err != ErrUserDisabled && err != ErrUnauthenticated {
		t.Fatalf("disabled session: %v", err)
	}
}

func TestRegisterDisabled(t *testing.T) {
	db := mysql.OpenTestDB(t)
	svc := NewService(db, false, time.Hour, 24*time.Hour)
	if _, _, err := svc.Register(context.Background(), "nobody", "correct horse battery staple", "127.0.0.1"); err != ErrRegistrationDisabled {
		t.Fatalf("got %v", err)
	}
}
