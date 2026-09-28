package adapters

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestBinanceCredentialDetection(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	edPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: edDER}))
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsaDER, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, secret string
		ed, valid    bool
	}{
		{"hmac", "plain-secret", false, true},
		{"ed25519", edPEM, true, true},
		{"broken PEM", "-----BEGIN PRIVATE KEY-----\nbroken\n-----END PRIVATE KEY-----", true, false},
		{"RSA PEM", string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: rsaDER})), true, false},
		{"encrypted PEM", "-----BEGIN ENCRYPTED PRIVATE KEY-----\nbroken\n-----END ENCRYPTED PRIVATE KEY-----", true, false},
		{"OpenSSH PEM", "-----BEGIN OPENSSH PRIVATE KEY-----\nbroken\n-----END OPENSSH PRIVATE KEY-----", true, false},
		{"public key PEM", strings.ReplaceAll(edPEM, "PRIVATE KEY", "PUBLIC KEY"), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key, ed, err := BinancePrivateKey(tc.secret)
			if ed != tc.ed || (err == nil) != tc.valid {
				t.Fatalf("ed %v, err %v", ed, err)
			}
			if tc.ed && tc.valid && !key.Equal(private) {
				t.Fatal("parsed a different Ed25519 private key")
			}
			creds := Credentials{APIKey: "key", APISecret: tc.secret}
			if err := CheckCredentials(PlatformBinance, StructureBinancePM, nil, creds); (err == nil) != tc.valid {
				t.Fatalf("save-time validation: %v", err)
			}
		})
	}
}
