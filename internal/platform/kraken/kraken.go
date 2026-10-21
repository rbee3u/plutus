// Package kraken implements read-only Kraken access for separate wallets and
// unified wallet. Field mappings: docs/platform/kraken/mapping.md. Runtime
// responses stay private; RecordRequest is the explicit recording path.
package kraken

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/httpclient"
)

// Account types of the kraken_separate and kraken_unified account structures. Multi-M exists only
// in separate wallets: in unified wallet the multi-collateral wallet is main itself.
const (
	TypeMain               adapter.AccountType = "main"
	TypeEarn               adapter.AccountType = "earn"
	TypeMultiM             adapter.AccountType = "multi_m"
	TypeCoinM              adapter.AccountType = "coin_m"
	TypeDerivativesHolding adapter.AccountType = "derivatives_holding"
)

var structureWords = map[adapter.AccountStructure]string{
	adapter.StructureKrakenUnified:  "unified_balances = true (Unified wallet)",
	adapter.StructureKrakenSeparate: "unified_balances = false (Separate wallets)",
}

// Options configures an adapter. Zero values select production defaults.
type Options struct {
	SpotHost     string
	FuturesHost  string
	WebSocketURL string
	HTTP         *http.Client
	Now          func() time.Time
	// HTTPFactory creates a client with the adapter timeout when HTTP is nil.
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

func (a *Adapter) client(creds adapter.Credentials) *client {
	return &client{a: a, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock().UTC() }

// structureOf reads the account structure from the documented unified_balances flag of the
// balances snapshot. A missing flag is unknown; it is never read as separate wallets.
func structureOf(s *snapshot) (adapter.AccountStructure, error) {
	if s.Flex.UnifiedBalances == nil {
		return "", adapter.Malformed("Kraken "+epBalancesSnapshot.String(), "flex_futures.unified_balances")
	}
	if *s.Flex.UnifiedBalances {
		return adapter.StructureKrakenUnified, nil
	}
	return adapter.StructureKrakenSeparate, nil
}

// Platform wallet settings can change after the identity is created.
func checkStructure(s *snapshot, structure adapter.AccountStructure) error {
	actual, err := structureOf(s)
	if err != nil {
		return err
	}
	if actual != structure {
		return adapter.StructureMismatch(adapter.PlatformKraken, structureWords[actual], structure)
	}
	return nil
}

// Validate matches supplied key groups by customer reference. Futures-only keys
// may use accountUid when iiban is absent. Structure checks require futures
// access; Spot-only validation cannot perform them.
func (a *Adapter) Validate(ctx context.Context, creds adapter.Credentials, structure adapter.AccountStructure) (adapter.ValidationResult, error) {
	if !adapter.ValidStructure(adapter.PlatformKraken, structure) {
		return adapter.ValidationResult{}, adapter.UnsupportedStructure(adapter.PlatformKraken, structure)
	}
	c := a.client(creds)
	if err := adapter.CheckCredentials(adapter.PlatformKraken, structure, nil, c.creds); err != nil {
		return adapter.ValidationResult{}, err
	}
	var spotRef, futuresRef string
	if c.creds.APIKey != "" {
		var key struct {
			IBAN string `json:"iban"`
		}
		if err := c.call(ctx, epAPIKeyInfo, nil, &key); err != nil {
			return adapter.ValidationResult{}, err
		}
		if spotRef = normalizeRef(key.IBAN); spotRef == "" {
			return adapter.ValidationResult{}, adapter.Malformed("Kraken "+epAPIKeyInfo.String(), "iban")
		}
	}
	if c.creds.FuturesKey() != "" {
		var key struct {
			IBAN string `json:"iiban"`
		}
		if err := c.call(ctx, epFuturesKey, nil, &key); err != nil {
			return adapter.ValidationResult{}, err
		}
		futuresRef = normalizeRef(key.IBAN)
		if futuresRef == "" {
			return adapter.ValidationResult{}, adapter.Malformed("Kraken "+epFuturesKey.String(), "iiban")
		}
		s, err := c.balancesSnapshot(ctx)
		if err != nil {
			return adapter.ValidationResult{}, err
		}
		if err := checkStructure(&s, structure); err != nil {
			return adapter.ValidationResult{}, err
		}
	}
	if spotRef != "" && futuresRef != "" && spotRef != futuresRef {
		return adapter.ValidationResult{}, adapter.NewError(adapter.ClassAuthentication, "The Kraken spot and futures keys belong to different customer accounts.")
	}
	if spotRef != "" {
		return adapter.ValidationResult{ExternalRef: spotRef}, nil
	}
	return adapter.ValidationResult{ExternalRef: futuresRef}, nil
}

// normalizeRef removes the spaces Kraken may print inside a customer reference.
func normalizeRef(ref string) string { return strings.ToUpper(strings.Join(strings.Fields(ref), "")) }

// run holds what one Collect call shares across its accounts.
type run struct {
	earnMetadata map[string]earnStrategy
	a            *Adapter
	c            *client
	structure    adapter.AccountStructure
	assets       assetNames
	prices       spotPrices
	// Earn-only runs do not need a wallet snapshot.
	snapshot *snapshot
	// open caches the open positions and their contracts, shared by main or Multi-M and Coin-M.
	open *openBook
}

// Collect checks structure through the futures snapshot when wallet accounts
// are requested. Earn-only collection skips it because earn spans both structures.
func (a *Adapter) Collect(ctx context.Context, creds adapter.Credentials, structure adapter.AccountStructure, accounts []adapter.AccountType) (adapter.Batch, error) {
	if !adapter.ValidStructure(adapter.PlatformKraken, structure) {
		return adapter.Batch{}, adapter.UnsupportedStructure(adapter.PlatformKraken, structure)
	}
	c := a.client(creds)
	if err := adapter.CheckCredentials(adapter.PlatformKraken, structure, accounts, c.creds); err != nil {
		return adapter.Batch{}, err
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
			return adapter.Batch{}, err
		}
		if err := checkStructure(&s, structure); err != nil {
			return adapter.Batch{}, err
		}
		r.snapshot = &s
	}
	reports, err := adapter.CollectAccounts(ctx, accounts, a.now, func(ctx context.Context, accountType adapter.AccountType) (adapter.AccountReport, error) {
		if !adapter.ValidAccountType(structure, accountType) {
			return adapter.AccountReport{}, adapter.UnknownAccountType(adapter.PlatformKraken, structure, accountType)
		}
		if assetsErr != nil {
			return adapter.AccountReport{}, assetsErr
		}
		return r.account(ctx, accountType)
	})
	if err != nil {
		return adapter.Batch{}, err
	}
	return adapter.Batch{
		CollectedAt: a.now(),
		Reports:     reports,
		Prices:      adapter.QuotePrices(reports, priceQuotes, r.prices.lookup, pricedAt),
	}, nil
}

func needsSnapshot(accounts []adapter.AccountType) bool {
	for _, accountType := range accounts {
		if accountType != TypeEarn {
			return true
		}
	}
	return false
}

func (r *run) account(ctx context.Context, accountType adapter.AccountType) (adapter.AccountReport, error) {
	switch accountType { //nolint:exhaustive // Kraken supports only its declared account types, not dual investment.
	case TypeMain:
		if r.structure == adapter.StructureKrakenUnified {
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
		return adapter.AccountReport{}, adapter.UnknownAccountType(adapter.PlatformKraken, r.structure, accountType)
	}
}
