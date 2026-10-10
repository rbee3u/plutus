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

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/httpclient"
)

// Account Types of the bybit_uta_2_0 Account Structure.
const (
	TypeDualInvestment                      = adapters.TypeDualInvestment
	TypeUnified        adapters.AccountType = "unified"
	TypeFunding        adapters.AccountType = "funding"
	TypeEarn           adapters.AccountType = "earn"
)

// Only statuses 5 and 6 are supported; other names explain structure failures.
var accountStatuses = map[int]string{
	1: "a classic account",
	3: "Unified Trading Account 1.0",
	4: "Unified Trading Account 1.0 Pro",
	5: "Unified Trading Account 2.0",
	6: "Unified Trading Account 2.0 Pro",
}

var marginModes = map[string]adapters.MarginMode{
	"ISOLATED_MARGIN":  adapters.MarginBybitIsolated,
	"REGULAR_MARGIN":   adapters.MarginBybitCross,
	"PORTFOLIO_MARGIN": adapters.MarginBybitPM,
}

// Options configures an Adapter. Zero values select production defaults.
type Options struct {
	Host string
	HTTP *http.Client
	Now  func() time.Time
	// HTTPFactory creates a client with the Adapter timeout when HTTP is nil.
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

func (a *Adapter) client(creds adapters.Credentials) *client {
	return &client{http: a.http, host: a.host, clock: a.clock, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock.now().UTC() }

func (a *Adapter) Validate(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	c := a.client(creds)
	mode, err := readMarginMode(ctx, c, structure)
	if err != nil {
		return adapters.ValidationResult{}, err
	}
	var key struct {
		UserID json.Number `json:"userID"`
	}
	if err := c.call(ctx, epAPIKey, nil, &key); err != nil {
		return adapters.ValidationResult{}, err
	}
	uid := strings.TrimSpace(key.UserID.String())
	if uid == "" {
		return adapters.ValidationResult{}, adapters.Malformed(epAPIKey.String(), "userID")
	}
	return adapters.ValidationResult{ExternalRef: uid, MarginMode: mode}, nil
}

// Recheck on validation and every run: Platform settings can change externally.
func readMarginMode(ctx context.Context, c *client, structure adapters.AccountStructure) (adapters.MarginMode, error) {
	if structure != adapters.StructureBybitUTA2 {
		return "", adapters.UnsupportedStructure(adapters.PlatformBybit, structure)
	}
	var info struct {
		UnifiedMarginStatus *int   `json:"unifiedMarginStatus"`
		MarginMode          string `json:"marginMode"`
	}
	if err := c.call(ctx, epAccountInfo, nil, &info); err != nil {
		return "", err
	}
	if info.UnifiedMarginStatus == nil {
		return "", adapters.Malformed(epAccountInfo.String(), "unifiedMarginStatus")
	}
	if status := *info.UnifiedMarginStatus; status != 5 && status != 6 {
		name, ok := accountStatuses[status]
		if !ok {
			name = "an undocumented account status"
		}
		return "", adapters.StructureMismatch(adapters.PlatformBybit, fmt.Sprintf("%s (unifiedMarginStatus %d)", name, status), structure)
	}
	reported := strings.TrimSpace(info.MarginMode)
	if reported == "" {
		return "", adapters.Malformed(epAccountInfo.String(), "marginMode")
	}
	mode, ok := marginModes[reported]
	if !ok {
		return "", adapters.UnknownMarginMode(adapters.PlatformBybit, reported)
	}
	return mode, nil
}

func (a *Adapter) Collect(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	c := a.client(creds)
	mode, err := readMarginMode(ctx, c, structure)
	if err != nil {
		return adapters.Batch{}, err
	}
	// Read prices before reports to satisfy at-or-before valuation lookup.
	prices, pricedAt := a.tickers(ctx, c)
	earnPair := adapters.CollectEarnPair(func(ctx context.Context) (adapters.AccountReport, adapters.AccountReport, error) {
		return a.earn(ctx, c, prices, slices.Contains(accounts, TypeEarn))
	})
	reports, err := adapters.CollectAccounts(ctx, accounts, a.now, func(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
		//nolint:exhaustive // Manual is application-owned and never enters private Adapter collection.
		switch accountType {
		case TypeUnified:
			return a.unified(ctx, c, mode)
		case TypeFunding:
			return a.funding(ctx, c)
		case TypeEarn, TypeDualInvestment:
			return earnPair(ctx, accountType)
		default:
			return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformBybit, structure, accountType)
		}
	})
	if err != nil {
		return adapters.Batch{}, err
	}
	return adapters.Batch{
		CollectedAt: a.now(),
		MarginMode:  mode,
		Reports:     reports,
		Prices:      adapters.QuotePrices(reports, quotes, prices.lookup, pricedAt),
	}, nil
}
