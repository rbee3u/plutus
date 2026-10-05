// Package cursor encodes and decodes the opaque keyset cursors used by
// Portfolio-scoped list endpoints.
package cursor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
)

// ErrInvalid reports a malformed cursor, a version or signature mismatch, or a
// cursor bound to another Portfolio.
var ErrInvalid = errors.New("cursor is invalid")

// payloadSize is one version byte, 16 Portfolio bytes, 8 timestamp bytes,
// 16 ID bytes, and a 32-byte HMAC-SHA256 tag.
const payloadSize = 73

// Cursor is one keyset position: the sort timestamp and the tie-breaking ID.
type Cursor struct {
	PortfolioID uuid.UUID
	TimeMicro   int64
	ID          uuid.UUID
}

// Codec binds the cursors of one list to their wire version and HMAC key.
type Codec struct {
	Version byte
	Key     []byte
}

// Encode serializes a cursor to its opaque base64url form.
func (c Codec) Encode(cur Cursor) (string, error) {
	payload := make([]byte, payloadSize)
	payload[0] = c.Version
	copy(payload[1:17], cur.PortfolioID[:])
	binary.BigEndian.PutUint64(payload[17:25], uint64(cur.TimeMicro)) // #nosec G115 -- Signed timestamp bits round-trip through the cursor wire format.
	copy(payload[25:41], cur.ID[:])
	mac := hmac.New(sha256.New, c.Key)
	if _, err := mac.Write(payload[:41]); err != nil {
		return "", err
	}
	copy(payload[41:], mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// Decode parses a cursor and verifies its signature and Portfolio binding.
func (c Codec) Decode(raw string, portfolioID uuid.UUID) (Cursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) != payloadSize || data[0] != c.Version {
		return Cursor{}, ErrInvalid
	}
	mac := hmac.New(sha256.New, c.Key)
	if _, err := mac.Write(data[:41]); err != nil {
		return Cursor{}, err
	}
	if !hmac.Equal(mac.Sum(nil), data[41:]) {
		return Cursor{}, ErrInvalid
	}
	var cur Cursor
	copy(cur.PortfolioID[:], data[1:17])
	cur.TimeMicro = int64(binary.BigEndian.Uint64(data[17:25])) // #nosec G115 -- Restore the signed timestamp encoded by Encode.
	copy(cur.ID[:], data[25:41])
	if cur.PortfolioID != portfolioID {
		return Cursor{}, ErrInvalid
	}
	return cur, nil
}
