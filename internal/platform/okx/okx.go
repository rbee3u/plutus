// Package okx implements read-only OKX access for the Unified account. Field
// mappings: docs/platform/okx/mapping.md. Runtime responses stay private;
// RecordRequest is the explicit recording path.
package okx

import (
	"context"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/httpclient"
)

// Account Types of the okx_unified Account Structure, the same in every Margin Mode.
const (
	TypeDualInvestment                      = adapters.TypeDualInvestment
	TypeTrading        adapters.AccountType = "trading"
	TypeFunding        adapters.AccountType = "funding"
	TypeEarn           adapters.AccountType = "earn"
)

// marginModes maps OKX's account level, the acctLv of account/config, to its Margin Mode. OKX
// calls the levels account modes; all four are the same Unified account with the same Accounts.
var marginModes = map[string]adapters.MarginMode{
	"1": adapters.MarginOKXSpot,
	"2": adapters.MarginOKXFutures,
	"3": adapters.MarginOKXMCM,
	"4": adapters.MarginOKXPM,
}

// pageInterval spaces two pages of an endpoint OKX limits to one request per second.
const pageInterval = 1100 * time.Millisecond

// Options configures an Adapter. Zero values select production defaults.
type Options struct {
	Host string
	HTTP *http.Client
	Now  func() time.Time
	// HTTPFactory creates a client with the Adapter timeout when HTTP is nil.
	HTTPFactory func(time.Duration) *http.Client
}

// Adapter is the OKX adapter. It is safe for concurrent use.
type Adapter struct {
	host             string
	http             *http.Client
	clock            *clock
	futuresContracts *futuresContractCache
	pace             time.Duration
}

// New constructs an OKX adapter; nil options select production defaults.
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
	return &Adapter{
		host:             strings.TrimSuffix(opts.Host, "/"),
		http:             opts.HTTP,
		clock:            &clock{now: opts.Now},
		futuresContracts: newFuturesContractCache(opts.Now),
		pace:             pageInterval,
	}
}

func (a *Adapter) client(creds adapters.Credentials) *client {
	return &client{http: a.http, host: a.host, clock: a.clock, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock.now().UTC() }

// wait holds the next page of a rate-limited endpoint for the adapter's pace.
func (a *Adapter) wait(ctx context.Context) error {
	if a.pace <= 0 {
		return nil
	}
	timer := time.NewTimer(a.pace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type accountConfig struct {
	UID     string `json:"uid"`
	MainUID string `json:"mainUid"`
	AcctLv  string `json:"acctLv"`
}

// readAccountConfig reads the account configuration: the account's own UID and its Margin
// Mode. The User can switch the level inside OKX at any time, so every
// validation and every Collection Run reads it afresh (ADR 0001).
func readAccountConfig(ctx context.Context, c *client) (accountConfig, adapters.MarginMode, error) {
	var rows []accountConfig
	if err := c.call(ctx, epConfig, &rows); err != nil {
		return accountConfig{}, "", err
	}
	if len(rows) == 0 {
		return accountConfig{}, "", adapters.Malformed(epConfig.String(), "the account configuration")
	}
	mode, err := marginModeOf(rows[0].AcctLv)
	if err != nil {
		return accountConfig{}, "", err
	}
	return rows[0], mode, nil
}

// marginModeOf maps an account level. A level OKX adds later is refused by name instead of being
// collected with a mapping written for another level.
func marginModeOf(level string) (adapters.MarginMode, error) {
	level = strings.TrimSpace(level)
	if level == "" {
		return "", adapters.Malformed(epConfig.String(), "acctLv")
	}
	mode, ok := marginModes[level]
	if !ok {
		return "", adapters.UnknownMarginMode(adapters.PlatformOKX, "acctLv "+level)
	}
	return mode, nil
}

// Validate accepts sub-account keys and returns their own UID and Margin Mode.
func (a *Adapter) Validate(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	if structure != adapters.StructureOKXUnified {
		return adapters.ValidationResult{}, adapters.UnsupportedStructure(adapters.PlatformOKX, structure)
	}
	cfg, mode, err := readAccountConfig(ctx, a.client(creds))
	if err != nil {
		return adapters.ValidationResult{}, err
	}
	uid := strings.TrimSpace(cfg.UID)
	if uid == "" {
		return adapters.ValidationResult{}, adapters.Malformed(epConfig.String(), "uid")
	}
	return adapters.ValidationResult{ExternalRef: uid, MarginMode: mode}, nil
}

func (a *Adapter) Collect(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	// Market terms are reused only within this Collection Run.
	run := *a
	run.futuresContracts = newFuturesContractCache(a.now)
	a = &run

	if structure != adapters.StructureOKXUnified {
		return adapters.Batch{}, adapters.UnsupportedStructure(adapters.PlatformOKX, structure)
	}
	c := a.client(creds)
	_, mode, err := readAccountConfig(ctx, c)
	if err != nil {
		return adapters.Batch{}, err
	}
	// Read prices before reports to satisfy at-or-before valuation lookup.
	lookup, pricedAt := a.tickers(ctx, c)
	usdPrice := func(symbol string) (*big.Rat, bool) { return adapters.USDPrice(symbol, quotes, lookup) }
	earnPair := adapters.CollectEarnPair(func(ctx context.Context) (adapters.AccountReport, adapters.AccountReport, error) {
		return a.earn(ctx, c, usdPrice)
	})
	reports, err := adapters.CollectAccounts(ctx, accounts, a.now, func(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
		//nolint:exhaustive // Manual is application-owned and outside the private Adapter report fixture.
		switch accountType {
		case TypeTrading:
			return a.trading(ctx, c, mode)
		case TypeFunding:
			return a.funding(ctx, c)
		case TypeEarn, TypeDualInvestment:
			return earnPair(ctx, accountType)
		default:
			return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformOKX, structure, accountType)
		}
	})
	if err != nil {
		return adapters.Batch{}, err
	}
	return adapters.Batch{
		CollectedAt: a.now(),
		MarginMode:  mode,
		Reports:     reports,
		Prices:      adapters.QuotePrices(reports, quotes, lookup, pricedAt),
	}, nil
}
