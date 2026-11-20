package notification

import (
	"math/big"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// maxMessageLength leaves room below Telegram's 4096-character limit.
const maxMessageLength = 4000

// formatMessages renders one message, or several chunks for a very large
// account set, without inventing a parent ratio or count.
func formatMessages(portfolioName string, loc *time.Location, evaluatedAt time.Time, alerts []leverageAlert) []string {
	if len(alerts) == 0 {
		return nil
	}
	if loc == nil {
		loc = time.UTC
	}
	blocks := make([]string, 0, len(alerts))
	for i := range alerts {
		blocks = append(blocks, formatAlert(portfolioName, loc, evaluatedAt, &alerts[i]))
	}
	chunks := make([]string, 0, 1)
	current := "High leverage"
	for _, block := range blocks {
		candidate := current + "\n\n" + block
		if len(candidate) > maxMessageLength && current != "High leverage" {
			chunks = append(chunks, current)
			current = "High leverage\n\n" + block
			continue
		}
		current = candidate
	}
	chunks = append(chunks, current+"\n\nAdd margin to reduce risk.")
	return chunks
}

func formatAlert(portfolioName string, loc *time.Location, evaluatedAt time.Time, alert *leverageAlert) string {
	segments := []string{alert.Platform}
	if alert.IdentityName != "" {
		segments = append(segments, alert.IdentityName)
	}
	segments = append(segments, alert.AccountName)
	path := portfolioName + " · " + strings.Join(segments, " / ")
	lines := []string{
		path,
		"Real leverage: " + formatLeverage(alert.Leverage) + " (threshold: above 3x)",
		"Adjusted equity: " + formatUSD(alert.AdjustedEquity),
		"Gross notional: " + formatUSD(alert.Notional),
		"Valuation point: " + formatTime(evaluatedAt, loc),
	}
	if alert.Stale {
		if alert.ReportObservedAt != nil {
			lines = append(lines, "Report stale since "+formatTime(*alert.ReportObservedAt, loc)+".")
		} else {
			lines = append(lines, "Report stale.")
		}
	}
	return strings.Join(lines, "\n")
}

// formatLeverage keeps at least two decimals and enough precision that a value
// strictly above three is visibly above three.
func formatLeverage(value *big.Rat) string {
	if value == nil {
		return "unknown"
	}
	for digits := 2; digits <= 8; digits++ {
		candidate := trimLeverage(value.FloatString(digits))
		parsed, ok := decimal.Parse(candidate)
		if ok && parsed.Cmp(big.NewRat(highLeverageThreshold, 1)) > 0 {
			return candidate + "x"
		}
	}
	return decimal.Compact(decimal.Format(value)) + "x"
}

func trimLeverage(value string) string {
	if strings.Contains(value, ".") {
		value = strings.TrimRight(value, "0")
		value = strings.TrimRight(value, ".")
	}
	index := strings.IndexByte(value, '.')
	if index < 0 {
		return value + ".00"
	}
	if missing := 2 - (len(value) - index - 1); missing > 0 {
		value += strings.Repeat("0", missing)
	}
	return value
}

// formatUSD renders a known USD amount with two decimals and grouping.
func formatUSD(value *big.Rat) string {
	if value == nil {
		return "unknown"
	}
	raw := value.FloatString(2)
	negative := strings.HasPrefix(raw, "-")
	if negative {
		raw = strings.TrimPrefix(raw, "-")
	}
	parts := strings.SplitN(raw, ".", 2)
	grouped := groupThousands(parts[0]) + "." + parts[1]
	if negative {
		return "-$" + grouped
	}
	return "$" + grouped
}

func groupThousands(value string) string {
	if len(value) <= 3 {
		return value
	}
	var out strings.Builder
	first := len(value) % 3
	if first == 0 {
		first = 3
	}
	out.WriteString(value[:first])
	for i := first; i < len(value); i += 3 {
		out.WriteByte(',')
		out.WriteString(value[i : i+3])
	}
	return out.String()
}

func formatTime(value time.Time, loc *time.Location) string {
	return value.In(loc).Format("2006-01-02 15:04") + " (" + loc.String() + ")"
}
