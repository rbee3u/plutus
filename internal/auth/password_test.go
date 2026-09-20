package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	ok, rehash, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok || rehash {
		t.Fatalf("verify got ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	ok, _, err = VerifyPassword("wrong password!!", hash)
	if err != nil || ok {
		t.Fatalf("wrong password accepted: ok=%v err=%v", ok, err)
	}
}

func TestPasswordPolicy(t *testing.T) {
	t.Parallel()

	if err := ValidatePassword("short"); err == nil {
		t.Fatal("expected policy error")
	}
}

func TestNormalizeUsername(t *testing.T) {
	t.Parallel()

	display, normalized, err := NormalizeUsername("  Alice  ")
	if err != nil {
		t.Fatal(err)
	}
	if display != "Alice" || normalized != "alice" {
		t.Fatalf("got display=%q normalized=%q", display, normalized)
	}
}
