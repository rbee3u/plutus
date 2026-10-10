// Package kraken implements read-only Kraken access for Separate wallets and
// Unified Wallet. Field mappings: docs/platform/kraken/mapping.md. Runtime
// responses stay private; RecordRequest is the explicit recording path.
package kraken

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/httpclient"
)

// Account Types of the kraken_separate and kraken_unified Account Structures. Multi-M exists only
// in Separate wallets: in Unified Wallet the multi-collateral wallet is Main itself.
const (
	TypeMain               adapters.AccountType = "main"
	TypeEarn               adapters.AccountType = "earn"
	TypeMultiM             adapters.AccountType = "multi_m"
	TypeCoinM              adapters.AccountType = "coin_m"
	TypeDerivativesHolding adapters.AccountType = "derivatives_holding"
)

var structureWords = map[adapters.AccountStructure]string{
	adapters.StructureKrakenUnified:  "unified_balances = true (Unified Wallet)",
	adapters.StructureKrakenSeparate: "unified_balances = false (Separate wallets)",
}

// Options configures an Adapter. Zero values select production defaults.
type Options struct {
	SpotHost     string
	FuturesHost  string
	WebSocketURL string
	HTTP         *http.Client
	Now          func() time.Time
	// HTTPFactory creates a client with the Adapter timeout when HTTP is nil.
	HTTPFactory func(time.Duration) *http.Client
}

// Adapter is the Kraken adapter. It is safe for concurrent use.
type Adapter struct {
	spotHost        string
	futuresHost     string
	wsURL           string
	http            *http.Client
	clock           func() time.Time
	nonces          *nonces
	captureSnapshot func([]byte)
}

// New constructs a Kraken adapter; nil options select production defaults.
func New(in *Options) *Adapter {
	opts := Options{}
	if in != nil {
		opts = *in
	}
	if opts.SpotHost == "" {
		opts.SpotHost = DefaultSpotHost
	}
	if opts.FuturesHost == "" {
		opts.FuturesHost = DefaultFuturesHost
	}
	if opts.WebSocketURL == "" {
		opts.WebSocketURL = DefaultWebSocketURL
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
		spotHost:    strings.TrimRight(opts.SpotHost, "/"),
		futuresHost: strings.TrimRight(opts.FuturesHost, "/"),
		wsURL:       opts.WebSocketURL,
		http:        opts.HTTP,
		clock:       opts.Now,
		nonces:      &nonces{},
	}
}

func (a *Adapter) client(creds adapters.Credentials) *client {
	return &client{a: a, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock().UTC() }

// structureOf reads the Account Structure from the documented unified_balances flag of the
// balances snapshot. A missing flag is unknown; it is never read as Separate wallets.
func structureOf(s *snapshot) (adapters.AccountStructure, error) {
	if s.Flex.UnifiedBalances == nil {
		return "", adapters.Malformed("Kraken "+epBalancesSnapshot.String(), "flex_futures.unified_balances")
	}
	if *s.Flex.UnifiedBalances {
		return adapters.StructureKrakenUnified, nil
	}
	return adapters.StructureKrakenSeparate, nil
}

// Platform wallet settings can change after the Identity is created.
func checkStructure(s *snapshot, structure adapters.AccountStructure) error {
	actual, err := structureOf(s)
	if err != nil {
		return err
	}
	if actual != structure {
		return adapters.StructureMismatch(adapters.PlatformKraken, structureWords[actual], structure)
	}
	return nil
}

// Validate matches supplied key groups by customer reference. Futures-only keys
// may use accountUid when iiban is absent. Structure checks require Futures
// access; Spot-only validation cannot perform them.
func (a *Adapter) Validate(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	if !adapters.ValidStructure(adapters.PlatformKraken, structure) {
		return adapters.ValidationResult{}, adapters.UnsupportedStructure(adapters.PlatformKraken, structure)
	}
	c := a.client(creds)
	if err := adapters.CheckCredentials(adapters.PlatformKraken, structure, nil, c.creds); err != nil {
		return adapters.ValidationResult{}, err
	}
	var spotRef, futuresRef string
	if c.creds.APIKey != "" {
		var key struct {
			IBAN string `json:"iban"`
		}
		if err := c.call(ctx, epAPIKeyInfo, nil, &key); err != nil {
			return adapters.ValidationResult{}, err
		}
		if spotRef = normalizeRef(key.IBAN); spotRef == "" {
			return adapters.ValidationResult{}, adapters.Malformed("Kraken "+epAPIKeyInfo.String(), "iban")
		}
	}
	if c.creds.FuturesKey() != "" {
		var key struct {
			IBAN string `json:"iiban"`
		}
		if err := c.call(ctx, epFuturesKey, nil, &key); err != nil {
			return adapters.ValidationResult{}, err
		}
		futuresRef = normalizeRef(key.IBAN)
		if futuresRef == "" {
			return adapters.ValidationResult{}, adapters.Malformed("Kraken "+epFuturesKey.String(), "iiban")
		}
		s, err := c.balancesSnapshot(ctx)
		if err != nil {
			return adapters.ValidationResult{}, err
		}
		if err := checkStructure(&s, structure); err != nil {
			return adapters.ValidationResult{}, err
		}
	}
	if spotRef != "" && futuresRef != "" && spotRef != futuresRef {
		return adapters.ValidationResult{}, adapters.NewError(adapters.ClassAuthentication, "The Kraken Spot and Futures keys belong to different customer accounts.")
	}
	if spotRef != "" {
		return adapters.ValidationResult{ExternalRef: spotRef}, nil
	}
	return adapters.ValidationResult{ExternalRef: futuresRef}, nil
}

// normalizeRef removes the spaces Kraken may print inside a customer reference.
func normalizeRef(ref string) string { return strings.ToUpper(strings.Join(strings.Fields(ref), "")) }

// run holds what one Collect call shares across its Accounts.
type run struct {
	earnMetadata map[string]earnStrategy
	a            *Adapter
	c            *client
	structure    adapters.AccountStructure
	assets       assetNames
	prices       spotPrices
	// Earn-only runs do not need a wallet snapshot.
	snapshot *snapshot
	// open caches the open positions and their contracts, shared by Main or Multi-M and Coin-M.
	open *openBook
}

// Collect checks structure through the Futures snapshot when wallet Accounts
// are requested. Earn-only collection skips it because Earn spans both structures.
func (a *Adapter) Collect(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	if !adapters.ValidStructure(adapters.PlatformKraken, structure) {
		return adapters.Batch{}, adapters.UnsupportedStructure(adapters.PlatformKraken, structure)
	}
	c := a.client(creds)
	if err := adapters.CheckCredentials(adapters.PlatformKraken, structure, accounts, c.creds); err != nil {
		return adapters.Batch{}, err
	}
	r := &run{a: a, c: c, structure: structure}
	// Market data comes first so its observation time precedes every report of the batch, and
	// the valuation's "latest price at or before the report" picks it.
	assets, assetsErr := readAssetNames(ctx, c)
	r.assets = assets
	r.prices = readSpotPrices(ctx, c, assets)
	pricedAt := a.now()
	if needsSnapshot(accounts) {
		s, err := c.balancesSnapshot(ctx)
		if err != nil {
			return adapters.Batch{}, err
		}
		if err := checkStructure(&s, structure); err != nil {
			return adapters.Batch{}, err
		}
		r.snapshot = &s
	}
	reports, err := adapters.CollectAccounts(ctx, accounts, a.now, func(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
		if !adapters.ValidAccountType(structure, accountType) {
			return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformKraken, structure, accountType)
		}
		if assetsErr != nil {
			return adapters.AccountReport{}, assetsErr
		}
		return r.account(ctx, accountType)
	})
	if err != nil {
		return adapters.Batch{}, err
	}
	return adapters.Batch{
		CollectedAt: a.now(),
		Reports:     reports,
		Prices:      adapters.QuotePrices(reports, priceQuotes, r.prices.lookup, pricedAt),
	}, nil
}

func needsSnapshot(accounts []adapters.AccountType) bool {
	for _, accountType := range accounts {
		if accountType != TypeEarn {
			return true
		}
	}
	return false
}

func (r *run) account(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
	switch accountType { //nolint:exhaustive // Kraken supports only its declared Account Types, not Dual Investment.
	case TypeMain:
		if r.structure == adapters.StructureKrakenUnified {
			return r.multiCollateral(ctx, TypeMain)
		}
		return r.separateMain(ctx)
	case TypeMultiM:
		return r.multiCollateral(ctx, TypeMultiM)
	case TypeCoinM:
		return r.coinM(ctx)
	case TypeDerivativesHolding:
		return r.derivativesHolding()
	case TypeEarn:
		return r.earn(ctx)
	default:
		return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformKraken, r.structure, accountType)
	}
}
