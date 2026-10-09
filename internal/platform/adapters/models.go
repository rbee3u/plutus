package adapters

import (
	"time"
)

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
	EarnDetailsCollected   bool
	EarnPositions          []EarnPosition
}

// Balance is one coin line within an Account Report.
type Balance struct {
	PlatformAssetSymbol string
	// Name is the Asset's full name when the Platform reports one; empty otherwise.
	Name                string
	Equity              string
	AvailableToWithdraw *string
	AvailableToTrade    *string
	Liability           *string
	Interest            *string
	UnrealizedPnl       *string
}

// FuturesContract describes the tradable contract behind a Position.
type FuturesContract struct {
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
	FuturesContract     FuturesContract
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
