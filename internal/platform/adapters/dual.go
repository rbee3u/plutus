package adapters

import (
	"context"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/shared/decimal"
)

const TypeDualInvestment AccountType = "dual_investment"

// DualPosition is one outstanding subscription. Principal is also represented in
// balances; the position is descriptive and never adds to account valuation.
type DualPosition struct {
	PositionKey          string
	ProductRef           *string
	OrderRef             *string
	BaseAssetSymbol      *string
	QuoteAssetSymbol     *string
	PrincipalAssetSymbol string
	Principal            string
	Direction            *string
	TargetPrice          *string
	ExpiryAt             *time.Time
	AnnualizedRate       *string
	RateKind             *string
	Status               string
	SubscribedAt         *time.Time
}

// MissingFields names unavailable product terms without making known principal unknown.
func (p *DualPosition) MissingFields() []string {
	var missing []string
	for _, field := range []struct {
		name   string
		absent bool
	}{
		{"base asset", p.BaseAssetSymbol == nil},
		{"quote asset", p.QuoteAssetSymbol == nil},
		{"direction", p.Direction == nil},
		{"target price", p.TargetPrice == nil},
		{"expiry", p.ExpiryAt == nil},
		{"annualized rate", p.AnnualizedRate == nil},
	} {
		if field.absent {
			missing = append(missing, field.name)
		}
	}
	return missing
}

// NormalizeDualPosition validates identity and principal, normalizes optional
// terms, and rejects contradictory direction/currency combinations.
func NormalizeDualPosition(where string, input *DualPosition) (DualPosition, error) {
	if input == nil {
		return DualPosition{}, Malformed(where, "dual position")
	}
	p := *input
	if p.PositionKey == "" || len(p.PositionKey) > 255 {
		return p, Malformed(where, "dual position id")
	}
	for _, ref := range []*string{p.ProductRef, p.OrderRef} {
		if ref != nil && len(*ref) > 255 {
			return p, Malformed(where, "dual product or order id")
		}
	}
	p.PrincipalAssetSymbol = strings.ToUpper(strings.TrimSpace(p.PrincipalAssetSymbol))
	if p.PrincipalAssetSymbol == "" {
		return p, Malformed(where, "dual investment currency")
	}
	principal, err := Decimal(where, "dual principal of "+p.PrincipalAssetSymbol, p.Principal)
	if err != nil {
		return p, err
	}
	if principal.Sign() <= 0 {
		return p, Malformed(where, "positive dual principal")
	}
	p.Principal = decimal.Format(principal)
	for _, symbol := range []**string{&p.BaseAssetSymbol, &p.QuoteAssetSymbol} {
		if *symbol != nil {
			*symbol = OptionalText(strings.ToUpper(**symbol))
		}
	}
	for _, field := range []struct {
		name     string
		value    **string
		positive bool
	}{
		{"target price", &p.TargetPrice, true}, {"annualized rate", &p.AnnualizedRate, false},
	} {
		if *field.value == nil {
			continue
		}
		value := strings.TrimSpace(**field.value)
		if value == "" {
			*field.value = nil
			continue
		}
		parsed, err := Decimal(where, field.name, value)
		if err != nil {
			return p, err
		}
		if parsed.Sign() < 0 || (field.positive && parsed.Sign() == 0) {
			return p, Malformed(where, field.name)
		}
		*field.value = decimal.FormatPtr(parsed)
	}
	if p.AnnualizedRate == nil {
		p.RateKind = nil
	} else if p.RateKind == nil || (*p.RateKind != "apr" && *p.RateKind != "apy" && *p.RateKind != "annualized_yield") {
		return p, Malformed(where, "annualized rate kind")
	}
	if p.ExpiryAt != nil && (p.ExpiryAt.Year() < 1970 || p.ExpiryAt.Year() > 9999) {
		return p, Malformed(where, "expiry")
	}
	if p.SubscribedAt != nil && (p.SubscribedAt.Year() < 1970 || p.SubscribedAt.Year() > 9999) {
		return p, Malformed(where, "dual subscription time")
	}
	if p.Direction != nil && *p.Direction != "buy_low" && *p.Direction != "sell_high" {
		return p, Malformed(where, "dual direction")
	}
	if p.Direction != nil {
		expected := p.QuoteAssetSymbol
		if *p.Direction == "sell_high" {
			expected = p.BaseAssetSymbol
		}
		if expected != nil && *expected != p.PrincipalAssetSymbol {
			return p, Malformed(where, "dual direction matching principal currency")
		}
	}
	if p.BaseAssetSymbol != nil && p.QuoteAssetSymbol != nil {
		if *p.BaseAssetSymbol == *p.QuoteAssetSymbol {
			return p, Malformed(where, "distinct dual currencies")
		}
		var direction string
		switch p.PrincipalAssetSymbol {
		case *p.BaseAssetSymbol:
			direction = "sell_high"
		case *p.QuoteAssetSymbol:
			direction = "buy_low"
		default:
			return p, Malformed(where, "dual principal currency matching the pair")
		}
		if p.Direction != nil && *p.Direction != direction {
			return p, Malformed(where, "dual direction matching principal currency")
		}
		p.Direction = &direction
	}
	if p.Status != "pending" && p.Status != "active" && p.Status != "settling" && p.Status != "redeeming" && p.Status != "refunding" {
		return p, Malformed(where, "dual status")
	}
	return p, nil
}

// DualExpiry reads milliseconds since epoch without treating missing terms as zero.
func DualExpiry(where, value string) (*time.Time, error) {
	return OptionalMillis(where, "dual expiry", value)
}

// CollectEarnPair shares auxiliary reads and their failures within one collection
// run. Only reports requested by CollectAccounts leave the adapter.
func CollectEarnPair(load func(context.Context) (AccountReport, AccountReport, error)) func(context.Context, AccountType) (AccountReport, error) {
	var loaded bool
	var earn, dual AccountReport
	var err error
	return func(ctx context.Context, kind AccountType) (AccountReport, error) {
		if !loaded {
			earn, dual, err = load(ctx)
			loaded = true
		}
		if kind == TypeDualInvestment {
			return dual, err
		}
		return earn, err
	}
}
