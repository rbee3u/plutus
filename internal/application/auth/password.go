package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/rbee3u/plutus/internal/shared/apperr"
)

const (
	passwordMinRunes = 8
	passwordMaxRunes = 128
	argonTime        = 3
	argonMemoryKiB   = 64 * 1024
	argonThreads     = 2
	argonKeyLen      = 32
	argonSaltLen     = 16
)

// ErrPasswordPolicy is returned when a password does not satisfy length or complexity requirements.
var ErrPasswordPolicy = apperr.Invalid("password must be 8 to 128 characters and include uppercase, lowercase, and a number or symbol")

// ErrSamePassword is returned when the new password is the same as the current password.
var ErrSamePassword = apperr.Invalid("new password must differ from current password")

// HashPassword returns a PHC-encoded Argon2id hash.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return encodePHC(salt, hash, argonTime, argonMemoryKiB, argonThreads), nil
}

// ValidatePassword enforces the documented password length and complexity.
func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < passwordMinRunes || n > passwordMaxRunes {
		return ErrPasswordPolicy
	}
	var hasUpper, hasLower, hasNumberOrSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || !unicode.IsLetter(r):
			hasNumberOrSymbol = true
		}
	}
	if !hasUpper || !hasLower || !hasNumberOrSymbol {
		return ErrPasswordPolicy
	}
	return nil
}

// VerifyPassword checks a password against a PHC string.
func VerifyPassword(password, encoded string) (bool, bool, error) {
	params, salt, hash, err := decodePHC(encoded)
	if err != nil {
		return false, false, err
	}
	computed := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(hash))) // #nosec G115 -- The decoded digest comes from a stored PHC hash in VARCHAR(255).
	ok := subtle.ConstantTimeCompare(hash, computed) == 1
	rehash := params.time != argonTime || params.memory != argonMemoryKiB || params.threads != argonThreads || len(hash) != argonKeyLen
	return ok, rehash, nil
}

type argonParams struct {
	time    uint32
	memory  uint32
	threads uint8
}

func encodePHC(salt, hash []byte, time, memory uint32, threads uint8) string {
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, time, threads, b64.EncodeToString(salt), b64.EncodeToString(hash))
}

func decodePHC(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, errors.New("invalid password hash")
	}
	var params argonParams
	for _, item := range strings.Split(parts[3], ",") {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return argonParams{}, nil, nil, errors.New("invalid password hash parameters")
		}
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return argonParams{}, nil, nil, errors.New("invalid password hash parameters")
		}
		switch key {
		case "m":
			params.memory = uint32(n)
		case "t":
			params.time = uint32(n)
		case "p":
			if n == 0 || n > 255 {
				return argonParams{}, nil, nil, errors.New("invalid password hash parallelism")
			}
			params.threads = uint8(n)
		}
	}
	if params.time == 0 || params.threads == 0 {
		return argonParams{}, nil, nil, errors.New("invalid password hash parameters")
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, errors.New("invalid password hash salt")
	}
	hash, err := b64.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, errors.New("invalid password hash digest")
	}
	return params, salt, hash, nil
}
