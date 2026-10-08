package adapters

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// EarnPosition describes value already represented in Earn Balances. References
// identify a source product aggregate, strategy, or individual position.
type EarnPosition struct {
	PlatformPositionRef string       `json:"platformPositionRef"`
	ProductRef          *string      `json:"productRef"`
	ProductName         *string      `json:"productName"`
	Type                string       `json:"type"`
	Granularity         string       `json:"granularity"`
	AmountBasis         string       `json:"amountBasis"`
	Assets              []EarnAmount `json:"assets"`
	Rates               []EarnRate   `json:"rates"`
	Rewards             []EarnReward `json:"rewards"`
	SubscribedAt        *time.Time   `json:"subscribedAt"`
	MaturityAt          *time.Time   `json:"maturityAt"`
	Status              *string      `json:"status"`
	Details             []EarnDetail `json:"details"`
	Problems            []string     `json:"problems"`
}

type EarnAmount struct {
	Symbol      string  `json:"symbol"`
	AssetSymbol *string `json:"assetSymbol,omitempty"`
	Amount      string  `json:"amount"`
}

type EarnRate struct {
	Value     string  `json:"value"`
	Upper     *string `json:"upper"`
	Kind      string  `json:"kind"`
	Qualifier string  `json:"qualifier"`
	Label     string  `json:"label"`
	Symbol    *string `json:"symbol"`
}

type EarnReward struct {
	Symbol string `json:"symbol"`
	Amount string `json:"amount"`
	Kind   string `json:"kind"`
}

// EarnDetail contains normalized explanatory facts, never raw response data.
type EarnDetail struct {
	Label string     `json:"label"`
	Value *string    `json:"value"`
	At    *time.Time `json:"at"`
}

func NewEarnPosition(ref, kind, grain, basis string) EarnPosition {
	return EarnPosition{
		PlatformPositionRef: ref, Type: kind, Granularity: grain, AmountBasis: basis,
		Assets: []EarnAmount{}, Rates: []EarnRate{}, Rewards: []EarnReward{}, Details: []EarnDetail{}, Problems: []string{},
	}
}

// Optional metadata failures retain known holdings and never expose source values.
func (p *EarnPosition) OptionalDecimal(label, value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	n, ok := decimal.Parse(value)
	if !decimal.Valid(value) || !ok {
		p.Problems = append(p.Problems, label+" is invalid.")
		return nil
	}
	return decimal.FormatPtr(n)
}

func (p *EarnPosition) AddRate(value, kind, qualifier, label string, symbol *string) {
	if n := p.OptionalDecimal(label, value); n != nil {
		p.Rates = append(p.Rates, EarnRate{Value: *n, Kind: kind, Qualifier: qualifier, Label: label, Symbol: symbol})
	}
}

func (p *EarnPosition) AddReward(symbol, value, kind string) {
	if n := p.OptionalDecimal(kind+" rewards", value); n != nil {
		if strings.TrimSpace(symbol) == "" {
			p.Problems = append(p.Problems, kind+" reward currency is unavailable.")
			return
		}
		p.Rewards = append(p.Rewards, EarnReward{Symbol: symbol, Amount: *n, Kind: kind})
	}
}

func (p *EarnPosition) AddDetail(label, value string) {
	if text := OptionalText(value); text != nil {
		p.Details = append(p.Details, EarnDetail{Label: label, Value: text})
	}
}

func (p *EarnPosition) Millis(label, value string) *time.Time {
	at, err := OptionalMillis("Earn position", label, value)
	if err != nil {
		p.Problems = append(p.Problems, label+" is invalid.")
		return nil
	}
	return at
}

func (p *EarnPosition) AddMillisDetail(label, value string) {
	if at := p.Millis(label, value); at != nil {
		p.Details = append(p.Details, EarnDetail{Label: label, At: at})
	}
}

func (p *EarnPosition) RequiredMillis(label, value string) *time.Time {
	at := p.Millis(label, value)
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) == "0" {
		p.Problems = append(p.Problems, label+" was not supplied.")
	}
	return at
}

// NormalizeEarnPosition enforces the core identity and quantity boundary. Optional
// fields have already been normalized by their Platform mapping.
func NormalizeEarnPosition(p *EarnPosition) error {
	if p.PlatformPositionRef == "" || len(p.PlatformPositionRef) > 128 ||
		!slices.Contains([]string{"flexible", "fixed", "bonded", "timed", "on_chain"}, p.Type) ||
		!slices.Contains([]string{"product", "position", "strategy"}, p.Granularity) ||
		!slices.Contains([]string{"principal", "total", "allocated"}, p.AmountBasis) || len(p.Assets) == 0 {
		return NewError(ClassNormalization, "Earn position identity, type, or amounts are unavailable.")
	}
	seen := map[string]bool{}
	for i := range p.Assets {
		a := &p.Assets[i]
		a.Symbol = strings.ToUpper(strings.TrimSpace(a.Symbol))
		n, ok := decimal.Parse(a.Amount)
		if !decimal.Valid(a.Amount) || !ok || n.Sign() <= 0 || a.Symbol == "" || seen[a.Symbol] {
			return NewError(ClassNormalization, "Earn position has invalid or duplicate invested amounts.")
		}
		seen[a.Symbol] = true
		a.Amount = decimal.Format(n)
	}
	slices.SortFunc(p.Assets, func(a, b EarnAmount) int { return strings.Compare(a.Symbol, b.Symbol) })
	for _, r := range p.Rates {
		if !decimal.Valid(r.Value) || (r.Upper != nil && !decimal.Valid(*r.Upper)) || (r.Kind != "apr" && r.Kind != "apy") {
			return NewError(ClassNormalization, "Earn position has invalid normalized rate metadata.")
		}
	}
	for _, r := range p.Rewards {
		if r.Symbol == "" || !decimal.Valid(r.Amount) {
			return NewError(ClassNormalization, "Earn position has invalid normalized rewards.")
		}
	}
	return nil
}

// CheckEarnPositions ensures descriptive detail cannot silently omit valued funds.
func CheckEarnPositions(positions []EarnPosition, balances []Balance) error {
	totals := Totals{}
	seen := map[string]bool{}
	for i := range positions {
		p := &positions[i]
		if err := NormalizeEarnPosition(p); err != nil {
			return err
		}
		if seen[p.PlatformPositionRef] {
			return NewError(ClassNormalization, "Earn position references are duplicated.")
		}
		seen[p.PlatformPositionRef] = true
		for _, a := range p.Assets {
			if err := totals.Add("Earn positions", a.Symbol, decimal.ParseOrZero(a.Amount)); err != nil {
				return err
			}
		}
	}
	known := totals.Balances()
	if len(known) != len(balances) {
		return NewError(ClassNormalization, "Earn details do not match the reported Balances.")
	}
	for _, b := range balances {
		amount, ok := decimal.Parse(b.Equity)
		if !decimal.Valid(b.Equity) || !ok {
			return NewError(ClassNormalization, "Earn Balance has an invalid amount.")
		}
		found := false
		for i, k := range known {
			if strings.EqualFold(k.Symbol, b.Symbol) && decimal.ParseOrZero(k.Equity).Cmp(amount) == 0 {
				known = slices.Delete(known, i, i+1)
				found = true
				break
			}
		}
		if !found {
			return NewError(ClassNormalization, fmt.Sprintf("Earn details do not match the %s Balance.", b.Symbol))
		}
	}
	return nil
}
