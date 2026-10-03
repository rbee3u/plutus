// Package adapters defines the read-only Platform port, the Platform capability catalog, and the
// helpers every live adapter shares.
package adapters

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Platform is a supported exchange code.
type Platform string

// AccountStructure is how a Platform arranges an Identity's Accounts. It decides which Account
// Types exist, is declared when the Identity is created, and never changes afterwards.
type AccountStructure string

// MarginMode is the margin regime a Platform applies to a whole trading account within an Account
// Structure. Plutus observes it from the Platform; only some Account Structures have one.
type MarginMode string

// AccountType is one collectable account within an Account Structure.
type AccountType string

// Supported Platforms.
const (
	PlatformBinance Platform = "binance"
	PlatformOKX     Platform = "okx"
	PlatformBybit   Platform = "bybit"
	PlatformKraken  Platform = "kraken"
)

// Supported Account Structures.
const (
	StructureBinanceClassic AccountStructure = "binance_classic"
	StructureBinancePM      AccountStructure = "binance_pm"
	StructureOKXUnified     AccountStructure = "okx_unified"
	StructureBybitUTA2      AccountStructure = "bybit_uta_2_0"
	StructureKrakenSeparate AccountStructure = "kraken_separate"
	StructureKrakenUnified  AccountStructure = "kraken_unified"
)

// Supported Margin Modes. Binance and Kraken structures have none.
const (
	MarginOKXSpot       MarginMode = "okx_spot"
	MarginOKXFutures    MarginMode = "okx_futures"
	MarginOKXMCM        MarginMode = "okx_mcm"
	MarginOKXPM         MarginMode = "okx_pm"
	MarginBybitIsolated MarginMode = "bybit_isolated"
	MarginBybitCross    MarginMode = "bybit_cross"
	MarginBybitPM       MarginMode = "bybit_pm"
)

// Error classes returned by adapters.
const (
	ClassAuthentication = "authentication"
	ClassAuthorization  = "authorization"
	ClassRateLimited    = "rate_limited"
	ClassTemporary      = "temporary"
	ClassUnsupported    = "unsupported"
	ClassNormalization  = "normalization"
)

// Report statuses.
const (
	StatusOK     = "ok"
	StatusFailed = "failed"
)

// Position sides and quantity units.
const (
	SideLong        = "long"
	SideShort       = "short"
	UnitContracts   = "contracts"
	UnitBase        = "base"
	UnitQuote       = "quote"
	KindSpot        = "spot"
	KindLinearPerp  = "linear_perpetual"
	KindInversePerp = "inverse_perpetual"
	KindLinearDeliv = "linear_delivery"
	KindInverseDlv  = "inverse_delivery"
)

// ErrNoAdapter is returned when no adapter is registered for a Platform.
var ErrNoAdapter = errors.New("no adapter is registered for the platform")

// Error is a classified adapter failure. Detail never contains raw payloads or credentials.
type Error struct {
	Class  string
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Class
	}
	return fmt.Sprintf("%s: %s", e.Class, e.Detail)
}

// NewError constructs a classified adapter error.
func NewError(class, detail string) *Error {
	return &Error{Class: class, Detail: detail}
}

// Classify returns the adapter error class of err, or ClassTemporary for unknown errors.
func Classify(err error) (class, detail string) {
	var adapterErr *Error
	if errors.As(err, &adapterErr) {
		return adapterErr.Class, adapterErr.Detail
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ClassTemporary, "The platform did not answer within the timeout."
	}
	if errors.Is(err, ErrNoAdapter) {
		return ClassUnsupported, ErrNoAdapter.Error()
	}
	return ClassTemporary, "The platform call failed."
}

// StopsScheduling reports whether an error class must set the Connection invalid.
func StopsScheduling(class string) bool {
	return class == ClassAuthentication || class == ClassAuthorization
}

// Retryable reports whether an error class may be retried within the same run.
func Retryable(class string) bool {
	return class == ClassRateLimited || class == ClassTemporary
}

// Credentials is the decrypted read-only API key material passed to an adapter call.
type Credentials struct {
	APIKey     string
	APISecret  string
	Passphrase string
	Futures    *KeyPair
}

// KeyPair is an optional additional API credential group.
type KeyPair struct {
	APIKey    string
	APISecret string
}

// FuturesKey returns the configured Futures key, or empty when that group is absent.
func (c Credentials) FuturesKey() string {
	if c.Futures == nil {
		return ""
	}
	return c.Futures.APIKey
}

// FuturesSecret returns the configured Futures secret, or empty when that group is absent.
func (c Credentials) FuturesSecret() string {
	if c.Futures == nil {
		return ""
	}
	return c.Futures.APISecret
}

// CheckCredentials enforces complete key pairs and the selected Accounts' requirements.
func CheckCredentials(platform Platform, structure AccountStructure, accounts []AccountType, creds Credentials) error {
	spotKey, spotSecret := strings.TrimSpace(creds.APIKey), strings.TrimSpace(creds.APISecret)
	futuresKey, futuresSecret := strings.TrimSpace(creds.FuturesKey()), strings.TrimSpace(creds.FuturesSecret())
	if (spotKey == "") != (spotSecret == "") || (futuresKey == "") != (futuresSecret == "") {
		return NewError(ClassAuthentication, "Supply both the API key and secret for each credential group.")
	}
	if platform != PlatformKraken {
		if spotKey == "" || futuresKey != "" {
			return NewError(ClassAuthentication, "This platform requires one API key and secret.")
		}
		if platform == PlatformBinance {
			if _, _, err := BinancePrivateKey(spotSecret); err != nil {
				return err
			}
		}
		return nil
	}
	if !ValidStructure(platform, structure) {
		return NewError(ClassUnsupported, "The Kraken Account Structure is not supported.")
	}
	if spotKey == "" && futuresKey == "" {
		return NewError(ClassAuthentication, "Supply at least one Kraken credential group.")
	}
	for _, account := range accounts {
		if (account == "main" || account == "earn") && spotKey == "" {
			return NewError(ClassAuthentication, "Selected Main and Earn Accounts require Kraken Spot credentials.")
		}
		if account != "earn" && futuresKey == "" {
			return NewError(ClassAuthentication, "Selected Main and derivatives Accounts require Kraken Futures credentials; Main uses them to verify the wallet structure.")
		}
	}
	return nil
}

// ValidationResult is the outcome of one read-only validation.
type ValidationResult struct {
	// ExternalRef is the Platform's own reference of the account the key reaches.
	ExternalRef string
	// MarginMode is the Margin Mode the Platform reports, empty for a structure that has none.
	MarginMode MarginMode
}

// Batch is one normalized collection result for one Identity.
type Batch struct {
	ObservedAt time.Time
	// MarginMode is the Margin Mode the Platform reported at the start of the run, empty for a
	// structure that has none.
	MarginMode MarginMode
	Reports    []AccountReport
	Prices     []Price
}

// AccountReport is the normalized state of one Account, or its structured failure.
type AccountReport struct {
	AccountType            AccountType
	Status                 string
	ErrorClass             string
	ErrorDetail            string
	ObservedAt             time.Time
	Equity                 *string
	AdjustedEquity         *string
	AvailableEquity        *string
	InitialMarginRatio     *string
	MaintenanceMarginRatio *string
	UnifiedMarginRatio     *string
	Balances               []Balance
	Positions              []Position
	DualPositions          []DualPosition
}

// Balance is one coin line within an Account Report.
type Balance struct {
	Symbol string
	// Name is the Asset's full name when the Platform reports one; empty otherwise.
	Name                string
	Equity              string
	AvailableToWithdraw *string
	AvailableToTrade    *string
	Liability           *string
	Interest            *string
	UnrealizedPnl       *string
}

// Instrument describes the tradable contract behind a Position.
type Instrument struct {
	Symbol           string
	Kind             string
	BaseSymbol       string
	QuoteSymbol      string
	SettlementSymbol string
	ExpiryAt         *time.Time
	ContractSize     *string
}

// Position is one open position within an Account Report.
type Position struct {
	PlatformPositionRef string
	Instrument          Instrument
	Side                string
	PositionMode        string
	MarginMode          string
	MarginCurrency      string
	MarginBasis         string
	Quantity            string
	QuantityUnit        string
	EntryPrice          *string
	MarkPrice           *string
	LiquidationPrice    *string
	UnrealizedPnl       *string
	Leverage            *string
	Margin              *string
}

// Price is one Platform market price for an Asset symbol in a quote symbol.
type Price struct {
	AssetSymbol string
	Price       string
	QuoteSymbol string
	ObservedAt  time.Time
}

// Adapter collects normalized facts for one Identity through read-only
// endpoints. Both calls first confirm that the account still has the
// Identity's Account Structure (ADR 0001) and read its Margin Mode (ADR 0001);
// a different structure or an unknown Margin Mode fails with ClassUnsupported.
type Adapter interface {
	// Validate checks the key and returns the Platform's own account reference and Margin Mode.
	Validate(ctx context.Context, creds Credentials, structure AccountStructure) (ValidationResult, error)
	// Collect returns one AccountReport per requested Account with balances, positions, and prices.
	Collect(ctx context.Context, creds Credentials, structure AccountStructure, accounts []AccountType) (Batch, error)
}

// Registry maps Platform codes to adapters.
type Registry struct {
	byPlatform map[Platform]Adapter
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byPlatform: map[Platform]Adapter{}}
}

// Register installs an adapter for a Platform.
func (r *Registry) Register(platform Platform, adapter Adapter) {
	if r == nil || adapter == nil {
		return
	}
	r.byPlatform[platform] = adapter
}

// Lookup returns the adapter for a Platform.
func (r *Registry) Lookup(platform Platform) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.byPlatform[platform]
	return adapter, ok
}

// Available reports whether an adapter exists for the Platform.
func (r *Registry) Available(platform Platform) bool {
	_, ok := r.Lookup(platform)
	return ok
}

// PlatformInfo describes one supported Platform for the API and the UI.
type PlatformInfo struct {
	ID               Platform
	Name             string
	AdapterAvailable bool
	Structures       []StructureInfo
}

// StructureInfo describes one Account Structure, the Account Types it holds, and the Margin
// Modes the Platform may report within it.
type StructureInfo struct {
	ID           AccountStructure
	Name         string
	AccountTypes []AccountTypeInfo
	MarginModes  []MarginModeInfo
}

// AccountTypeInfo describes one Account Type.
type AccountTypeInfo struct {
	ID   AccountType
	Name string
}

// MarginModeInfo describes one Margin Mode.
type MarginModeInfo struct {
	ID   MarginMode
	Name string
}

type structureSpec struct {
	name        string
	types       []AccountTypeInfo
	marginModes []MarginModeInfo
}

var platformNames = map[Platform]string{
	PlatformBinance: "Binance",
	PlatformOKX:     "OKX",
	PlatformBybit:   "Bybit",
	PlatformKraken:  "Kraken",
}

// platformStructures lists the Account Structures offered per Platform, in display order.
var platformStructures = map[Platform][]AccountStructure{
	PlatformBinance: {StructureBinanceClassic, StructureBinancePM},
	PlatformOKX:     {StructureOKXUnified},
	PlatformBybit:   {StructureBybitUTA2},
	PlatformKraken:  {StructureKrakenSeparate, StructureKrakenUnified},
}

// OfferedPlatforms lists, in display order, every Platform that has at least one Account
// Structure. These are the Platforms the API offers, the fake adapter covers, and the seeds use.
func OfferedPlatforms() []Platform {
	ids := make([]Platform, 0, len(platformNames))
	for _, id := range []Platform{PlatformBinance, PlatformOKX, PlatformBybit, PlatformKraken} {
		if len(platformStructures[id]) > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

var structureSpecs = map[AccountStructure]structureSpec{
	StructureBinanceClassic: {name: "Classic Trading", types: []AccountTypeInfo{
		{ID: "spot", Name: "Spot"},
		{ID: "cross_margin", Name: "Cross Margin"},
		{ID: "isolated_margin", Name: "Isolated Margin"},
		{ID: "um_futures", Name: "USDⓈ-M Futures"},
		{ID: "cm_futures", Name: "COIN-M Futures"},
		{ID: "funding", Name: "Funding"},
		{ID: "earn", Name: "Earn"},
		{ID: TypeDualInvestment, Name: "Dual Investment"},
	}},
	StructureBinancePM: {name: "Portfolio Margin", types: []AccountTypeInfo{
		{ID: "spot", Name: "Spot"},
		{ID: "portfolio_margin", Name: "Portfolio Margin"},
		{ID: "funding", Name: "Funding"},
		{ID: "earn", Name: "Earn"},
		{ID: TypeDualInvestment, Name: "Dual Investment"},
	}},
	StructureOKXUnified: {
		name: "Unified account",
		types: []AccountTypeInfo{
			{ID: "trading", Name: "Trading"},
			{ID: "funding", Name: "Funding"},
			{ID: "earn", Name: "Earn"},
			{ID: TypeDualInvestment, Name: "Dual Investment"},
		},
		marginModes: []MarginModeInfo{
			{ID: MarginOKXSpot, Name: "Spot mode"},
			{ID: MarginOKXFutures, Name: "Futures mode"},
			{ID: MarginOKXMCM, Name: "Multi-currency margin"},
			{ID: MarginOKXPM, Name: "Portfolio margin"},
		},
	},
	StructureBybitUTA2: {
		name: "Unified Trading Account 2.0",
		types: []AccountTypeInfo{
			{ID: "unified", Name: "Unified"},
			{ID: "funding", Name: "Funding"},
			{ID: "earn", Name: "Earn"},
			{ID: TypeDualInvestment, Name: "Dual Investment"},
		},
		marginModes: []MarginModeInfo{
			{ID: MarginBybitIsolated, Name: "Isolated Margin"},
			{ID: MarginBybitCross, Name: "Cross Margin"},
			{ID: MarginBybitPM, Name: "Portfolio Margin"},
		},
	},
	StructureKrakenSeparate: {name: "Separate wallets", types: []AccountTypeInfo{
		{ID: "main", Name: "Main"},
		{ID: "multi_m", Name: "Multi-M"},
		{ID: "coin_m", Name: "Coin-M"},
		{ID: "derivatives_holding", Name: "Derivatives Holding"},
		{ID: "earn", Name: "Earn"},
	}},
	StructureKrakenUnified: {name: "Unified Wallet", types: []AccountTypeInfo{
		{ID: "main", Name: "Main"},
		{ID: "coin_m", Name: "Coin-M"},
		{ID: "derivatives_holding", Name: "Derivatives Holding"},
		{ID: "earn", Name: "Earn"},
	}},
}

// Platforms lists every offered Platform with its Account Structures, Account Types, and Margin
// Modes.
func (r *Registry) Platforms() []PlatformInfo {
	ids := OfferedPlatforms()
	out := make([]PlatformInfo, 0, len(ids))
	for _, id := range ids {
		info := PlatformInfo{ID: id, Name: platformNames[id], AdapterAvailable: r.Available(id)}
		for _, structure := range platformStructures[id] {
			spec := structureSpecs[structure]
			info.Structures = append(info.Structures, StructureInfo{
				ID:           structure,
				Name:         spec.name,
				AccountTypes: append([]AccountTypeInfo(nil), spec.types...),
				MarginModes:  append([]MarginModeInfo(nil), spec.marginModes...),
			})
		}
		out = append(out, info)
	}
	return out
}

// ValidPlatform reports whether the code is a supported Platform.
func ValidPlatform(platform Platform) bool {
	_, ok := platformNames[platform]
	return ok
}

// ValidStructure reports whether the Account Structure belongs to the Platform.
func ValidStructure(platform Platform, structure AccountStructure) bool {
	for _, s := range platformStructures[platform] {
		if s == structure {
			return true
		}
	}
	return false
}

// StructureName returns the display name of an Account Structure.
func StructureName(structure AccountStructure) string {
	if spec, ok := structureSpecs[structure]; ok {
		return spec.name
	}
	return string(structure)
}

// AccountTypes returns the Account Types of an Account Structure in display order.
func AccountTypes(structure AccountStructure) []AccountTypeInfo {
	spec, ok := structureSpecs[structure]
	if !ok {
		return nil
	}
	return append([]AccountTypeInfo(nil), spec.types...)
}

// AccountTypeIDs returns the Account Type codes of an Account Structure in display order.
func AccountTypeIDs(structure AccountStructure) []AccountType {
	types := AccountTypes(structure)
	out := make([]AccountType, 0, len(types))
	for _, t := range types {
		out = append(out, t.ID)
	}
	return out
}

// ValidAccountType reports whether the Account Type belongs to the Account Structure.
func ValidAccountType(structure AccountStructure, accountType AccountType) bool {
	for _, t := range AccountTypes(structure) {
		if t.ID == accountType {
			return true
		}
	}
	return false
}

// AccountTypeName returns the display name of an Account Type within an Account Structure.
func AccountTypeName(structure AccountStructure, accountType AccountType) string {
	for _, t := range AccountTypes(structure) {
		if t.ID == accountType {
			return t.Name
		}
	}
	return string(accountType)
}

// MarginModes returns the Margin Modes the Platform may report within an Account Structure.
func MarginModes(structure AccountStructure) []MarginModeInfo {
	spec, ok := structureSpecs[structure]
	if !ok {
		return nil
	}
	return append([]MarginModeInfo(nil), spec.marginModes...)
}

// ValidMarginMode reports whether a Margin Mode belongs to the Account Structure. The empty
// Margin Mode is valid exactly for structures that have none.
func ValidMarginMode(structure AccountStructure, mode MarginMode) bool {
	modes := MarginModes(structure)
	if mode == "" {
		return len(modes) == 0
	}
	for _, m := range modes {
		if m.ID == mode {
			return true
		}
	}
	return false
}

// MarginModeName returns the display name of a Margin Mode.
func MarginModeName(mode MarginMode) string {
	for _, spec := range structureSpecs {
		for _, m := range spec.marginModes {
			if m.ID == mode {
				return m.Name
			}
		}
	}
	return string(mode)
}

// PlatformName returns the display name of a Platform.
func PlatformName(platform Platform) string {
	if name, ok := platformNames[platform]; ok {
		return name
	}
	return string(platform)
}
