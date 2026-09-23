// Package decimal holds exact arithmetic helpers for DECIMAL(65,30) strings.
package decimal

import (
	"math/big"
	"regexp"
	"strings"
)

const scale = 30

var pattern = regexp.MustCompile(`^-?[0-9]{1,35}(\.[0-9]{1,30})?$`)

// Valid reports whether value is a plain decimal literal the database accepts.
func Valid(value string) bool {
	return pattern.MatchString(strings.TrimSpace(value))
}

// Parse converts a decimal string into an exact rational. Empty strings are not numbers.
func Parse(value string) (*big.Rat, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	rat, ok := new(big.Rat).SetString(value)
	return rat, ok
}

// MustParse parses a literal that the caller knows is valid and returns zero otherwise.
func MustParse(value string) *big.Rat {
	rat, ok := Parse(value)
	if !ok {
		return new(big.Rat)
	}
	return rat
}

// Format renders an exact rational as a compact decimal string with at most 30 fraction digits.
func Format(value *big.Rat) string {
	if value == nil {
		return "0"
	}
	return Compact(value.FloatString(scale))
}

// Compact removes trailing fraction zeros and a dangling decimal point.
func Compact(value string) string {
	if value == "" || !strings.Contains(value, ".") {
		return value
	}
	trimmed := strings.TrimRight(value, "0")
	trimmed = strings.TrimRight(trimmed, ".")
	if trimmed == "" || trimmed == "-" || trimmed == "-0" {
		return "0"
	}
	return trimmed
}

// Zero returns a fresh zero rational.
func Zero() *big.Rat {
	return new(big.Rat)
}

// Add returns a + b treating nil as zero.
func Add(a, b *big.Rat) *big.Rat {
	if a == nil {
		a = new(big.Rat)
	}
	if b == nil {
		return new(big.Rat).Set(a)
	}
	return new(big.Rat).Add(a, b)
}

// Sub returns a - b treating nil as zero.
func Sub(a, b *big.Rat) *big.Rat {
	if a == nil {
		a = new(big.Rat)
	}
	if b == nil {
		return new(big.Rat).Set(a)
	}
	return new(big.Rat).Sub(a, b)
}

// Mul returns a × b; a nil operand yields zero.
func Mul(a, b *big.Rat) *big.Rat {
	if a == nil || b == nil {
		return new(big.Rat)
	}
	return new(big.Rat).Mul(a, b)
}

// Sign returns -1, 0, or 1 treating nil as zero.
func Sign(value *big.Rat) int {
	if value == nil {
		return 0
	}
	return value.Sign()
}

// Abs returns |value| treating nil as zero.
func Abs(value *big.Rat) *big.Rat {
	if value == nil {
		return new(big.Rat)
	}
	if value.Sign() < 0 {
		return new(big.Rat).Neg(value)
	}
	return new(big.Rat).Set(value)
}

// FormatPtr renders an optional rational; nil stays nil so unknown is never written as zero.
func FormatPtr(value *big.Rat) *string {
	if value == nil {
		return nil
	}
	s := Format(value)
	return &s
}
