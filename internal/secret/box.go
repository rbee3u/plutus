// Package secret encrypts Connection credentials with AES-256-GCM.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	keySize           = 32
	nonceSize         = 12
	documentVersion   = 1
	currentKeyVersion = 1
)

var (
	// ErrInvalidKey is returned when the master key is not 32 bytes.
	ErrInvalidKey = errors.New("master key is invalid")
	// ErrInvalidCredentials is returned when required credential fields are empty.
	ErrInvalidCredentials = errors.New("connection credentials are invalid")
	// ErrOpenFailed is returned when ciphertext cannot be authenticated.
	ErrOpenFailed = errors.New("credential ciphertext could not be opened")
)

// Credentials is the write-only secret document.
type Credentials struct {
	APIKey     string
	APISecret  string
	Passphrase string
}

type document struct {
	V          int    `json:"v"`
	APIKey     string `json:"apiKey"`
	APISecret  string `json:"apiSecret"`
	Passphrase string `json:"passphrase,omitempty"`
}

// Box seals and opens Connection credentials.
type Box struct {
	aead cipher.AEAD
}

// NewBox constructs an AES-256-GCM box from a 32-byte master key.
func NewBox(key []byte) (*Box, error) {
	if len(key) != keySize {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if aead.NonceSize() != nonceSize {
		return nil, fmt.Errorf("unexpected GCM nonce size %d", aead.NonceSize())
	}
	return &Box{aead: aead}, nil
}

// CurrentKeyVersion is the key version written for new ciphertext.
func CurrentKeyVersion() uint16 {
	return currentKeyVersion
}

// Seal encrypts credentials. AAD binds Connection ID, Portfolio ID, document version, and key version.
func (b *Box) Seal(connectionID, portfolioID [16]byte, keyVersion uint16, cred Credentials) (ciphertext, nonce []byte, err error) {
	if b == nil {
		return nil, nil, ErrInvalidKey
	}
	apiKey := strings.TrimSpace(cred.APIKey)
	apiSecret := strings.TrimSpace(cred.APISecret)
	if apiKey == "" || apiSecret == "" {
		return nil, nil, ErrInvalidCredentials
	}
	doc := document{
		V:          documentVersion,
		APIKey:     apiKey,
		APISecret:  apiSecret,
		Passphrase: strings.TrimSpace(cred.Passphrase),
	}
	plain, err := json.Marshal(doc) //nolint:gosec // G117: plaintext exists only until AES-GCM seal.
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = b.aead.Seal(nil, nonce, plain, additionalData(connectionID, portfolioID, documentVersion, keyVersion))
	return ciphertext, nonce, nil
}

// Open decrypts credentials and authenticates the bound identifiers.
func (b *Box) Open(connectionID, portfolioID [16]byte, keyVersion uint16, ciphertext, nonce []byte) (Credentials, error) {
	if b == nil {
		return Credentials{}, ErrInvalidKey
	}
	if len(nonce) != nonceSize || len(ciphertext) == 0 {
		return Credentials{}, ErrOpenFailed
	}
	plain, err := b.aead.Open(nil, nonce, ciphertext, additionalData(connectionID, portfolioID, documentVersion, keyVersion))
	if err != nil {
		return Credentials{}, ErrOpenFailed
	}
	var doc document
	if err := json.Unmarshal(plain, &doc); err != nil || doc.V != documentVersion || doc.APIKey == "" || doc.APISecret == "" {
		return Credentials{}, ErrOpenFailed
	}
	return Credentials{APIKey: doc.APIKey, APISecret: doc.APISecret, Passphrase: doc.Passphrase}, nil
}

func additionalData(connectionID, portfolioID [16]byte, schemaVersion uint32, keyVersion uint16) []byte {
	out := make([]byte, 16+16+4+2)
	copy(out[0:16], connectionID[:])
	copy(out[16:32], portfolioID[:])
	binary.BigEndian.PutUint32(out[32:36], schemaVersion)
	binary.BigEndian.PutUint16(out[36:38], keyVersion)
	return out
}
