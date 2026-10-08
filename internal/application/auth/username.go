package auth

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// NormalizeUsername trims, applies Unicode NFC, and returns display and login keys.
func NormalizeUsername(raw string) (display, normalized string, err error) {
	display = strings.TrimSpace(norm.NFC.String(raw))
	if display == "" {
		return "", "", ErrUsernameRequired
	}
	if len(display) > 191 {
		return "", "", ErrUsernameTooLong
	}
	normalized = cases.Fold().String(display)
	normalized = norm.NFC.String(normalized)
	return display, normalized, nil
}
