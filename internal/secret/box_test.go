package secret

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/google/uuid"
)

func TestSealOpenRoundTrip(t *testing.T) {
	t.Parallel()
	box := testBox(t)
	conn := uuid.Must(uuid.NewV7())
	port := uuid.Must(uuid.NewV7())
	cred := Credentials{APIKey: "key", APISecret: "secret", Passphrase: "phrase"}
	ct, nonce, err := box.Seal(conn, port, CurrentKeyVersion(), cred)
	if err != nil {
		t.Fatal(err)
	}
	got, err := box.Open(conn, port, CurrentKeyVersion(), ct, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if got != cred {
		t.Fatalf("got %+v want %+v", got, cred)
	}
}

func TestOpenRejectsWrongAAD(t *testing.T) {
	t.Parallel()
	box := testBox(t)
	conn := uuid.Must(uuid.NewV7())
	port := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	ct, nonce, err := box.Seal(conn, port, CurrentKeyVersion(), Credentials{APIKey: "key", APISecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := box.Open(other, port, CurrentKeyVersion(), ct, nonce); err != ErrOpenFailed {
		t.Fatalf("connection AAD: %v", err)
	}
	if _, err := box.Open(conn, other, CurrentKeyVersion(), ct, nonce); err != ErrOpenFailed {
		t.Fatalf("portfolio AAD: %v", err)
	}
	if _, err := box.Open(conn, port, CurrentKeyVersion()+1, ct, nonce); err != ErrOpenFailed {
		t.Fatalf("key version AAD: %v", err)
	}
}

func TestSealRejectsEmptyFields(t *testing.T) {
	t.Parallel()
	box := testBox(t)
	id := uuid.Must(uuid.NewV7())
	if _, _, err := box.Seal(id, id, CurrentKeyVersion(), Credentials{APIKey: " ", APISecret: "secret"}); err != ErrInvalidCredentials {
		t.Fatalf("empty api key: %v", err)
	}
	if _, _, err := box.Seal(id, id, CurrentKeyVersion(), Credentials{APIKey: "key", APISecret: ""}); err != ErrInvalidCredentials {
		t.Fatalf("empty api secret: %v", err)
	}
}

func TestNewBoxRejectsWrongKeySize(t *testing.T) {
	t.Parallel()
	if _, err := NewBox([]byte("short")); err != ErrInvalidKey {
		t.Fatalf("short key: %v", err)
	}
}

func TestSealUsesUniqueNonces(t *testing.T) {
	t.Parallel()
	box := testBox(t)
	id := uuid.Must(uuid.NewV7())
	cred := Credentials{APIKey: "key", APISecret: "secret"}
	_, n1, err := box.Seal(id, id, CurrentKeyVersion(), cred)
	if err != nil {
		t.Fatal(err)
	}
	_, n2, err := box.Seal(id, id, CurrentKeyVersion(), cred)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(n1, n2) {
		t.Fatal("nonces were reused")
	}
}

func testBox(t *testing.T) *Box {
	t.Helper()
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatal(err)
	}
	box, err := NewBox(key)
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func TestCredentialGroupsAndOldDocumentCompatibility(t *testing.T) {
	box := testBox(t)
	id := uuid.Must(uuid.NewV7())
	for _, creds := range []Credentials{
		{APIKey: "spot", APISecret: "secret", FuturesAPIKey: "futures", FuturesAPISecret: "other"},
		{FuturesAPIKey: "futures", FuturesAPISecret: "other"},
	} {
		ciphertext, nonce, err := box.Seal(id, id, CurrentKeyVersion(), creds)
		if err != nil {
			t.Fatal(err)
		}
		opened, err := box.Open(id, id, CurrentKeyVersion(), ciphertext, nonce)
		if err != nil || opened != creds {
			t.Fatal("credential groups did not round-trip")
		}
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatal(err)
	}
	old := []byte(`{"v":1,"apiKey":"old-key","apiSecret":"old-secret","passphrase":"old-phrase"}`)
	ciphertext := box.aead.Seal(nil, nonce, old, additionalData(id, id, 1, CurrentKeyVersion()))
	opened, err := box.Open(id, id, CurrentKeyVersion(), ciphertext, nonce)
	if err != nil || opened.APIKey != "old-key" || opened.FuturesAPIKey != "" {
		t.Fatal("existing credential document no longer decrypts")
	}
	for _, creds := range []Credentials{{FuturesAPIKey: "half"}, {APIKey: "spot", APISecret: "secret", FuturesAPISecret: "half"}} {
		if _, _, err := box.Seal(id, id, CurrentKeyVersion(), creds); err != ErrInvalidCredentials {
			t.Fatal("accepted an incomplete key pair")
		}
	}
}
