package adapter

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"strings"
)

// BinancePrivateKey recognizes an unencrypted PKCS#8 Ed25519 PEM. A secret without a PEM
// header is an HMAC secret. The error deliberately contains no credential material.
func BinancePrivateKey(secret string) (ed25519.PrivateKey, bool, error) {
	secret = strings.TrimSpace(secret)
	if !strings.HasPrefix(secret, "-----BEGIN") {
		return nil, false, nil
	}
	invalid := NewError(ClassAuthentication, "Binance requires an unencrypted PKCS#8 Ed25519 private key PEM, or an HMAC secret.")
	block, rest := pem.Decode([]byte(secret))
	if block == nil || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 || strings.TrimSpace(string(rest)) != "" {
		return nil, true, invalid
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, true, invalid
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, true, invalid
	}
	return key, true, nil
}
