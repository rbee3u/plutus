package projection

import (
	"math/big"
	"strings"
)

func parseRat(value string) (*big.Rat, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	rat, ok := new(big.Rat).SetString(value)
	return rat, ok
}

func compactDecimal(value string) string {
	if value == "" || !strings.Contains(value, ".") {
		return value
	}
	trimmed := strings.TrimRight(value, "0")
	trimmed = strings.TrimRight(trimmed, ".")
	if trimmed == "" || trimmed == "-" {
		return "0"
	}
	return trimmed
}

func formatRat(value *big.Rat) string {
	if value == nil {
		return ""
	}
	return compactDecimal(value.FloatString(30))
}

func addRat(a, b *big.Rat) *big.Rat {
	if a == nil {
		a = new(big.Rat)
	}
	if b == nil {
		return new(big.Rat).Set(a)
	}
	return new(big.Rat).Add(a, b)
}

func subRat(a, b *big.Rat) *big.Rat {
	if a == nil {
		a = new(big.Rat)
	}
	if b == nil {
		return new(big.Rat).Set(a)
	}
	return new(big.Rat).Sub(a, b)
}

func mulRat(a, b *big.Rat) *big.Rat {
	if a == nil || b == nil {
		return new(big.Rat)
	}
	return new(big.Rat).Mul(a, b)
}

func ratSign(value *big.Rat) int {
	if value == nil {
		return 0
	}
	return value.Sign()
}

func absRat(value *big.Rat) *big.Rat {
	if value == nil {
		return new(big.Rat)
	}
	if value.Sign() < 0 {
		return new(big.Rat).Neg(value)
	}
	return new(big.Rat).Set(value)
}
