package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"

	"github.com/google/uuid"
)

const cursorVersion byte = 1

var cursorHMACKey = []byte("plutus-audit-cursor-v1")

type listCursor struct {
	PortfolioID uuid.UUID
	OccurredAt  int64
	ID          uuid.UUID
}

func encodeCursor(c listCursor) (string, error) {
	payload := make([]byte, 73)
	payload[0] = cursorVersion
	copy(payload[1:17], c.PortfolioID[:])
	binary.BigEndian.PutUint64(payload[17:25], uint64(c.OccurredAt))
	copy(payload[25:41], c.ID[:])
	mac := hmac.New(sha256.New, cursorHMACKey)
	if _, err := mac.Write(payload[:41]); err != nil {
		return "", err
	}
	copy(payload[41:], mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(raw string, portfolioID uuid.UUID) (listCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) != 73 || data[0] != cursorVersion {
		return listCursor{}, ErrInvalidCursor
	}
	payload := data[:41]
	sum := data[41:]
	mac := hmac.New(sha256.New, cursorHMACKey)
	if _, err := mac.Write(payload); err != nil {
		return listCursor{}, err
	}
	if !hmac.Equal(sum, mac.Sum(nil)) {
		return listCursor{}, ErrInvalidCursor
	}
	var cursor listCursor
	copy(cursor.PortfolioID[:], payload[1:17])
	cursor.OccurredAt = int64(binary.BigEndian.Uint64(payload[17:25]))
	copy(cursor.ID[:], payload[25:41])
	if cursor.PortfolioID != portfolioID {
		return listCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}
