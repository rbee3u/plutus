package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("Correct=123456")
	if err != nil {
		t.Fatal(err)
	}
	ok, rehash, err := VerifyPassword("Correct=123456", hash)
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

	if err := ValidatePassword("1234567"); err == nil {
		t.Fatal("expected policy error for 7 characters")
	}
	if err := ValidatePassword("abcdefg1!"); err == nil {
		t.Fatal("expected policy error for missing uppercase")
	}
	if err := ValidatePassword("ABCDEFG1!"); err == nil {
		t.Fatal("expected policy error for missing lowercase")
	}
	if err := ValidatePassword("Abcdefgh"); err == nil {
		t.Fatal("expected policy error for missing number or symbol")
	}
	if err := ValidatePassword("Abcdefg1"); err != nil {
		t.Fatalf("expected valid with number, got: %v", err)
	}
	if err := ValidatePassword("Abcdefg!"); err != nil {
		t.Fatalf("expected valid with symbol, got: %v", err)
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

func TestPasswordHashParameterBounds(t *testing.T) {
	t.Parallel()
	for _, params := range []string{"m=65536,t=3,p=0", "m=65536,t=3,p=256", "m=65536,t=3,p=257", "m=65536,t=3", "m=65536,t=0,p=2"} {
		t.Run(params, func(t *testing.T) {
			encoded := "$argon2id$v=19$" + params + "$c2FsdA$aGFzaA"
			if ok, _, err := VerifyPassword("password", encoded); err == nil || ok {
				t.Fatalf("invalid parameters accepted: ok=%v err=%v", ok, err)
			}
		})
	}
	params, _, _, err := decodePHC("$argon2id$v=19$m=65536,t=3,p=255$c2FsdA$aGFzaA")
	if err != nil || params.threads != 255 {
		t.Fatalf("valid uint8 boundary rejected: threads=%d err=%v", params.threads, err)
	}
}
