// Package textkey defines Unicode name and comparison-key normalization.
package textkey

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Name normalizes a display name and bounds characters, not UTF-8 bytes.
func Name(raw string, maxCharacters int) (string, bool) {
	name := norm.NFC.String(strings.TrimSpace(raw))
	return name, utf8.ValidString(name) && name != "" && utf8.RuneCountInString(name) <= maxCharacters && utf8.RuneCountInString(Key(name)) <= maxCharacters*3
}

// Key preserves accents while making canonically equivalent case variants equal.
func Key(name string) string { return norm.NFC.String(cases.Fold().String(name)) }
