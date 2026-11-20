package notification

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

func textPtr(value string) *string {
	return &value
}

func leverageAccount(platform adapter.Platform, accountType, equity string, source *uuid.UUID) valuation.AccountValuation {
	return valuation.AccountValuation{
		AccountID:      uuid.New(),
		AccountName:    adapter.AccountName(adapter.AccountType(accountType)),
		AccountType:    accountType,
		IdentityName:   "main",
		Platform:       string(platform),
		SourceReportID: source,
		AdjustedEquity: textPtr(equity),
		Freshness:      valuation.FreshnessFresh,
	}
}

func position(notional *string) financial.PositionRow {
	return financial.PositionRow{NotionalUSD: notional}
}

func TestSupportsRealLeverage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		platform    adapter.Platform
		accountType string
		want        bool
	}{
		{adapter.PlatformBinance, "portfolio_margin", true},
		{adapter.PlatformOKX, "trading", true},
		{adapter.PlatformBybit, "unified", true},
		{adapter.PlatformKraken, "main", true},
		{adapter.PlatformBinance, "spot", false},
		{adapter.PlatformBinance, "um_futures", false},
		{adapter.PlatformOKX, "funding", false},
		{adapter.PlatformKraken, "coin_m", false},
		{adapter.PlatformBybit, "earn", false},
		{adapter.PlatformKraken, "multi_m", false},
	}
	for _, tc := range cases {
		if got := supportsRealLeverage(tc.platform, tc.accountType); got != tc.want {
			t.Errorf("supportsRealLeverage(%s, %s) = %v, want %v", tc.platform, tc.accountType, got, tc.want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	report := uuid.New()
	other := uuid.New()
	cases := []struct {
		name      string
		account   valuation.AccountValuation
		positions []financial.PositionRow
		want      bool
	}{
		{
			name:      "above threshold",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", &report),
			positions: []financial.PositionRow{position(textPtr("324"))},
			want:      true,
		},
		{
			name:      "exactly three",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", &report),
			positions: []financial.PositionRow{position(textPtr("300"))},
		},
		{
			name:      "unrounded above three",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", &report),
			positions: []financial.PositionRow{position(textPtr("300.01"))},
			want:      true,
		},
		{
			name: "stale known value still warns",
			account: func() valuation.AccountValuation {
				a := leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", &report)
				a.Freshness = valuation.FreshnessStale
				return a
			}(),
			positions: []financial.PositionRow{position(textPtr("324"))},
			want:      true,
		},
		{
			name:      "no source report",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", nil),
			positions: []financial.PositionRow{position(textPtr("324"))},
		},
		{
			name:      "zero equity",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "0", &report),
			positions: []financial.PositionRow{position(textPtr("324"))},
		},
		{
			name:      "negative equity",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "-1", &report),
			positions: []financial.PositionRow{position(textPtr("324"))},
		},
		{
			name:      "invalid equity",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "abc", &report),
			positions: []financial.PositionRow{position(textPtr("324"))},
		},
		{
			name:      "unknown notional",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", &report),
			positions: []financial.PositionRow{position(textPtr("324")), position(nil)},
		},
		{
			name:      "negative notional",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", &report),
			positions: []financial.PositionRow{position(textPtr("-1"))},
		},
		{
			name:      "invalid notional",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", &report),
			positions: []financial.PositionRow{position(textPtr("abc"))},
		},
		{
			name:      "empty positions",
			account:   leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", &report),
			positions: []financial.PositionRow{},
		},
		{
			name:      "unsupported scope",
			account:   leverageAccount(adapter.PlatformBinance, "spot", "100", &report),
			positions: []financial.PositionRow{position(textPtr("324"))},
		},
		{
			name:      "sums every position",
			account:   leverageAccount(adapter.PlatformOKX, "trading", "100", &report),
			positions: []financial.PositionRow{position(textPtr("200")), position(textPtr("124"))},
			want:      true,
		},
		{
			name:      "unrelated source report still qualifies",
			account:   leverageAccount(adapter.PlatformBybit, "unified", "100", &other),
			positions: []financial.PositionRow{position(textPtr("400"))},
			want:      true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			alert, ok := evaluate(tc.account, tc.positions)
			if ok != tc.want {
				t.Fatalf("evaluate() = %v, want %v", ok, tc.want)
			}
			if !ok {
				return
			}
			if alert.AccountName != tc.account.AccountName || alert.IdentityName != tc.account.IdentityName {
				t.Fatalf("alert labels: %+v", alert)
			}
			if alert.Platform == "" {
				t.Fatal("alert platform must be a display label")
			}
		})
	}
}

func TestEvaluateDoesNotRound(t *testing.T) {
	t.Parallel()

	report := uuid.New()
	account := leverageAccount(adapter.PlatformBinance, "portfolio_margin", "100", &report)
	alert, ok := evaluate(account, []financial.PositionRow{position(textPtr("300.0001"))})
	if !ok {
		t.Fatal("3.000001 must warn")
	}
	if alert.Leverage.Cmp(big.NewRat(3000001, 1000000)) != 0 {
		t.Fatalf("unexpected leverage: %s", alert.Leverage.RatString())
	}
	if alert.AdjustedEquity.Cmp(big.NewRat(100, 1)) != 0 || alert.Notional.Cmp(big.NewRat(3000001, 10000)) != 0 {
		t.Fatalf("unexpected amounts: equity=%s notional=%s", alert.AdjustedEquity.RatString(), alert.Notional.RatString())
	}
}

func TestFormatLeverage(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"4":       "4.00x",
		"3.24":    "3.24x",
		"3.2":     "3.20x",
		"3.0001":  "3.0001x",
		"3.00001": "3.00001x",
	}
	for raw, want := range cases {
		value, ok := new(big.Rat).SetString(raw)
		if !ok {
			t.Fatalf("parse %s", raw)
		}
		if got := formatLeverage(value); got != want {
			t.Errorf("formatLeverage(%s) = %q, want %q", raw, got, want)
		}
	}
	if got := formatLeverage(nil); got != "unknown" {
		t.Fatalf("nil leverage: %q", got)
	}
}

func TestFormatUSD(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"1000":   "$1,000.00",
		"4000":   "$4,000.00",
		"1234.5": "$1,234.50",
		"0":      "$0.00",
		"-12.5":  "-$12.50",
	}
	for raw, want := range cases {
		value, ok := new(big.Rat).SetString(raw)
		if !ok {
			t.Fatalf("parse %s", raw)
		}
		if got := formatUSD(value); got != want {
			t.Errorf("formatUSD(%s) = %q, want %q", raw, got, want)
		}
	}
	if got := formatUSD(nil); got != "unknown" {
		t.Fatalf("nil usd: %q", got)
	}
}

func TestFormatMessages(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	at := time.Date(2026, 11, 19, 21, 30, 0, 0, loc)
	alerts := []leverageAlert{{
		AccountName:    "Portfolio margin",
		IdentityName:   "main",
		Platform:       "Binance",
		Leverage:       big.NewRat(4, 1),
		AdjustedEquity: big.NewRat(1000, 1),
		Notional:       big.NewRat(4000, 1),
	}}
	messages := formatMessages("default", loc, at, alerts)
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d", len(messages))
	}
	want := "High leverage\n\ndefault · Binance / main / Portfolio margin\n" +
		"Real leverage: 4.00x (threshold: above 3x)\n" +
		"Adjusted equity: $1,000.00\n" +
		"Gross notional: $4,000.00\n" +
		"Valuation point: 2026-11-19 21:30 (Asia/Shanghai)\n\n" +
		"Add margin to reduce risk."
	if messages[0] != want {
		t.Fatalf("message:\n%s\nwant:\n%s", messages[0], want)
	}
}

func TestFormatMessagesStaleAndMultiple(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	at := time.Date(2026, 11, 19, 21, 30, 0, 0, loc)
	observed := time.Date(2026, 11, 19, 18, 0, 0, 0, loc)
	alerts := []leverageAlert{
		{
			AccountName:      "Portfolio margin",
			IdentityName:     "main",
			Platform:         "Binance",
			Leverage:         big.NewRat(324, 100),
			AdjustedEquity:   big.NewRat(1000, 1),
			Notional:         big.NewRat(3240, 1),
			Stale:            true,
			ReportObservedAt: &observed,
		},
		{
			AccountName:    "Unified",
			IdentityName:   "sub",
			Platform:       "Bybit",
			Leverage:       big.NewRat(42, 10),
			AdjustedEquity: big.NewRat(500, 1),
			Notional:       big.NewRat(2100, 1),
		},
	}
	messages := formatMessages("default", loc, at, alerts)
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d", len(messages))
	}
	message := messages[0]
	for _, part := range []string{
		"default · Binance / main / Portfolio margin",
		"Real leverage: 3.24x (threshold: above 3x)",
		"Report stale since 2026-11-19 18:00 (Asia/Shanghai).",
		"default · Bybit / sub / Unified",
		"Real leverage: 4.20x (threshold: above 3x)",
	} {
		if !strings.Contains(message, part) {
			t.Fatalf("missing %q in:\n%s", part, message)
		}
	}
	if strings.Count(message, "Add margin to reduce risk.") != 1 {
		t.Fatalf("action line must appear once:\n%s", message)
	}
}
