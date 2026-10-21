package auth

import "github.com/rbee3u/plutus/internal/shared/textkey"

// NormalizeUsername returns the display name and its exact case-folded login key.
func NormalizeUsername(raw string) (display, normalized string, err error) {
	display, valid := textkey.Name(raw, 191)
	if display == "" {
		return "", "", ErrUsernameRequired
	}
	if !valid {
		return "", "", ErrUsernameTooLong
	}
	return display, textkey.Key(display), nil
}
