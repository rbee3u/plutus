// Package adapters defines the read-only Platform port and the Platform capability catalog.
package adapters

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Platform is a supported exchange code.
type Platform string

// AccountMode is the Platform account structure an Identity uses.
type AccountMode string

// AccountType is one collectable account within an Account Mode.
type AccountType string

// Supported Platforms.
const (
	PlatformBinance Platform = "binance"
	PlatformOKX     Platform = "okx"
	PlatformBybit   Platform = "bybit"
)

// Supported Account Modes.
const (
	ModeBinancePM AccountMode = "binance_pm"
	ModeOKXMCM    AccountMode = "okx_mcm"
	ModeOKXPM     AccountMode = "okx_pm"
	ModeBybitUTA2 AccountMode = "bybit_uta_2_0"
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
}

// ValidationResult is the outcome of one read-only validation call.
type ValidationResult struct {
	ExternalRef string
}

// Batch is one normalized collection result for one Identity.
type Batch struct {
	ObservedAt time.Time
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
}

// Balance is one coin line within an Account Report.
type Balance struct {
	Symbol              string
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

// Adapter collects normalized facts for one Identity through read-only endpoints.
type Adapter interface {
	// Capabilities lists the Account Types this adapter can collect per Account Mode.
	Capabilities() map[AccountMode][]AccountType
	// Validate performs one read-only call and returns the Platform's own identity ref.
	Validate(ctx context.Context, creds Credentials, mode AccountMode) (ValidationResult, error)
	// Collect returns one AccountReport per requested Account with balances, positions, and prices.
	Collect(ctx context.Context, creds Credentials, mode AccountMode, accounts []AccountType) (Batch, error)
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
	Modes            []ModeInfo
}

// ModeInfo describes one Account Mode and the Account Types it can hold.
type ModeInfo struct {
	ID           AccountMode
	Name         string
	AccountTypes []AccountTypeInfo
}

// AccountTypeInfo describes one Account Type.
type AccountTypeInfo struct {
	ID   AccountType
	Name string
}

type modeSpec struct {
	name  string
	types []AccountTypeInfo
}

var platformNames = map[Platform]string{
	PlatformBinance: "Binance",
	PlatformOKX:     "OKX",
	PlatformBybit:   "Bybit",
}

var platformModes = map[Platform][]AccountMode{
	PlatformBinance: {ModeBinancePM},
	PlatformOKX:     {ModeOKXMCM, ModeOKXPM},
	PlatformBybit:   {ModeBybitUTA2},
}

var modeSpecs = map[AccountMode]modeSpec{
	ModeBinancePM: {name: "Portfolio Margin", types: []AccountTypeInfo{
		{ID: "portfolio_margin", Name: "Portfolio Margin"},
		{ID: "spot", Name: "Spot"},
		{ID: "funding", Name: "Funding"},
		{ID: "earn", Name: "Earn"},
	}},
	ModeOKXMCM: {name: "Multi-currency margin", types: []AccountTypeInfo{
		{ID: "trading", Name: "Trading"},
		{ID: "funding", Name: "Funding"},
		{ID: "earn", Name: "Earn"},
	}},
	ModeOKXPM: {name: "Portfolio margin", types: []AccountTypeInfo{
		{ID: "trading", Name: "Trading"},
		{ID: "funding", Name: "Funding"},
		{ID: "earn", Name: "Earn"},
	}},
	ModeBybitUTA2: {name: "Unified Trading Account 2.0", types: []AccountTypeInfo{
		{ID: "unified", Name: "Unified"},
		{ID: "funding", Name: "Funding"},
		{ID: "earn", Name: "Earn"},
	}},
}

// Platforms lists every supported Platform with its Account Modes and Account Types.
func (r *Registry) Platforms() []PlatformInfo {
	ids := make([]Platform, 0, len(platformNames))
	for id := range platformNames {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]PlatformInfo, 0, len(ids))
	for _, id := range ids {
		info := PlatformInfo{ID: id, Name: platformNames[id], AdapterAvailable: r.Available(id)}
		for _, mode := range platformModes[id] {
			spec := modeSpecs[mode]
			info.Modes = append(info.Modes, ModeInfo{ID: mode, Name: spec.name, AccountTypes: append([]AccountTypeInfo(nil), spec.types...)})
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

// ValidMode reports whether the Account Mode belongs to the Platform.
func ValidMode(platform Platform, mode AccountMode) bool {
	for _, m := range platformModes[platform] {
		if m == mode {
			return true
		}
	}
	return false
}

// AccountTypes returns the Account Types of an Account Mode in display order.
func AccountTypes(mode AccountMode) []AccountTypeInfo {
	spec, ok := modeSpecs[mode]
	if !ok {
		return nil
	}
	return append([]AccountTypeInfo(nil), spec.types...)
}

// ValidAccountType reports whether the Account Type belongs to the Account Mode.
func ValidAccountType(mode AccountMode, accountType AccountType) bool {
	for _, t := range AccountTypes(mode) {
		if t.ID == accountType {
			return true
		}
	}
	return false
}

// AccountTypeName returns the display name of an Account Type within a mode.
func AccountTypeName(mode AccountMode, accountType AccountType) string {
	for _, t := range AccountTypes(mode) {
		if t.ID == accountType {
			return t.Name
		}
	}
	return string(accountType)
}

// PlatformName returns the display name of a Platform.
func PlatformName(platform Platform) string {
	if name, ok := platformNames[platform]; ok {
		return name
	}
	return string(platform)
}
