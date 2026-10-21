// Package bybit implements read-only Bybit access for global Unified Trading
// Account 2.0. Field mappings: docs/platform/bybit/mapping.md. Runtime
// responses stay private; RecordRequest is the explicit recording path.
package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/httpclient"
)

// Account types of the bybit_uta_2_0 account structure.
const (
	TypeDualInvestment                     = adapter.TypeDualInvestment
	TypeUnified        adapter.AccountType = "unified"
	TypeFunding        adapter.AccountType = "funding"
	TypeEarn           adapter.AccountType = "earn"
)

// Only statuses 5 and 6 are supported; other names explain structure failures.
var accountStatuses = map[int]string{
	1: "a classic account",
	3: "Unified Trading Account 1.0",
	4: "Unified Trading Account 1.0 Pro",
	5: "Unified Trading Account 2.0",
	6: "Unified Trading Account 2.0 Pro",
}

var marginModes = map[string]adapter.MarginMode{
	"ISOLATED_MARGIN":  adapter.MarginBybitIsolated,
	"REGULAR_MARGIN":   adapter.MarginBybitCross,
	"PORTFOLIO_MARGIN": adapter.MarginBybitPM,
}

// Options configures an adapter. Zero values select production defaults.
type Options struct {
	Host string
	HTTP *http.Client
	Now  func() time.Time
	// HTTPFactory creates a client with the adapter timeout when HTTP is nil.
	HTTPFactory func(time.Duration) *http.Client
}

// Adapter is the Bybit adapter. It is safe for concurrent use.
type Adapter struct {
	host  string
	http  *http.Client
	clock *clock
}

// New constructs a Bybit adapter; nil options select production defaults.
func New(in *Options) *Adapter {
	opts := Options{}
	if in != nil {
		opts = *in
	}
	if opts.Host == "" {
		opts.Host = DefaultHost
	}
	if opts.HTTP == nil {
		factory := opts.HTTPFactory
		if factory == nil {
			factory = httpclient.NewIPv4
		}
		opts.HTTP = factory(requestTimeout)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Adapter{host: strings.TrimSuffix(opts.Host, "/"), http: opts.HTTP, clock: &clock{now: opts.Now}}
}

func (a *Adapter) client(creds adapter.Credentials) *client {
	return &client{http: a.http, host: a.host, clock: a.clock, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock.now().UTC() }

func (a *Adapter) Validate(ctx context.Context, creds adapter.Credentials, structure adapter.AccountStructure) (adapter.ValidationResult, error) {
	c := a.client(creds)
	mode, err := readMarginMode(ctx, c, structure)
	if err != nil {
		return adapter.ValidationResult{}, err
	}
	var key struct {
		UserID json.Number `json:"userID"`
	}
	if err := c.call(ctx, epAPIKey, nil, &key); err != nil {
		return adapter.ValidationResult{}, err
	}
	uid := strings.TrimSpace(key.UserID.String())
	if uid == "" {
		return adapter.ValidationResult{}, adapter.Malformed(epAPIKey.String(), "userID")
	}
	return adapter.ValidationResult{ExternalRef: uid, MarginMode: mode}, nil
}

// Recheck on validation and every run: platform settings can change externally.
func readMarginMode(ctx context.Context, c *client, structure adapter.AccountStructure) (adapter.MarginMode, error) {
	if structure != adapter.StructureBybitUTA2 {
		return "", adapter.UnsupportedStructure(adapter.PlatformBybit, structure)
	}
	var info struct {
		UnifiedMarginStatus *int   `json:"unifiedMarginStatus"`
		MarginMode          string `json:"marginMode"`
	}
	if err := c.call(ctx, epAccountInfo, nil, &info); err != nil {
		return "", err
	}
	if info.UnifiedMarginStatus == nil {
		return "", adapter.Malformed(epAccountInfo.String(), "unifiedMarginStatus")
	}
	if status := *info.UnifiedMarginStatus; status != 5 && status != 6 {
		name, ok := accountStatuses[status]
		if !ok {
			name = "an undocumented account status"
		}
		return "", adapter.StructureMismatch(adapter.PlatformBybit, fmt.Sprintf("%s (unifiedMarginStatus %d)", name, status), structure)
	}
	reported := strings.TrimSpace(info.MarginMode)
	if reported == "" {
		return "", adapter.Malformed(epAccountInfo.String(), "marginMode")
	}
	mode, ok := marginModes[reported]
	if !ok {
		return "", adapter.UnknownMarginMode(adapter.PlatformBybit, reported)
	}
	return mode, nil
}

func (a *Adapter) Collect(ctx context.Context, creds adapter.Credentials, structure adapter.AccountStructure, accounts []adapter.AccountType) (adapter.Batch, error) {
	c := a.client(creds)
	mode, err := readMarginMode(ctx, c, structure)
	if err != nil {
		return adapter.Batch{}, err
	}
	// Read prices before reports to satisfy at-or-before valuation lookup.
	prices, pricedAt := a.tickers(ctx, c)
	earnPair := adapter.CollectEarnPair(func(ctx context.Context) (adapter.AccountReport, adapter.AccountReport, error) {
		return a.earn(ctx, c, prices, slices.Contains(accounts, TypeEarn))
	})
	reports, err := adapter.CollectAccounts(ctx, accounts, a.now, func(ctx context.Context, accountType adapter.AccountType) (adapter.AccountReport, error) {
		switch accountType {
		case TypeUnified:
			return a.unified(ctx, c, mode)
		case TypeFunding:
			return a.funding(ctx, c)
		case TypeEarn, TypeDualInvestment:
			return earnPair(ctx, accountType)
		default:
			return adapter.AccountReport{}, adapter.UnknownAccountType(adapter.PlatformBybit, structure, accountType)
		}
	})
	if err != nil {
		return adapter.Batch{}, err
	}
	return adapter.Batch{
		CollectedAt: a.now(),
		MarginMode:  mode,
		Reports:     reports,
		Prices:      adapter.QuotePrices(reports, quotes, prices.lookup, pricedAt),
	}, nil
}
