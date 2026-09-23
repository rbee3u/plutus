package jobs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"

	"github.com/google/uuid"
)

const cursorVersion byte = 2

var cursorHMACKey = []byte("plutus-job-cursor-v2")

type listCursor struct {
	PortfolioID uuid.UUID
	CreatedAt   int64
	ID          uuid.UUID
}

func encodeCursor(c listCursor) (string, error) {
	payload := make([]byte, 73)
	payload[0] = cursorVersion
	copy(payload[1:17], c.PortfolioID[:])
	binary.BigEndian.PutUint64(payload[17:25], uint64(c.CreatedAt))
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
	mac := hmac.New(sha256.New, cursorHMACKey)
	if _, err := mac.Write(data[:41]); err != nil {
		return listCursor{}, err
	}
	if !hmac.Equal(mac.Sum(nil), data[41:]) {
		return listCursor{}, ErrInvalidCursor
	}
	var c listCursor
	copy(c.PortfolioID[:], data[1:17])
	c.CreatedAt = int64(binary.BigEndian.Uint64(data[17:25]))
	copy(c.ID[:], data[25:41])
	if c.PortfolioID != portfolioID {
		return listCursor{}, ErrInvalidCursor
	}
	return c, nil
}
