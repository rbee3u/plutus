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
	CollectedAt time.Time
	// MarginMode is the Margin Mode the Platform reported at the start of the run, empty for a
	// structure that has none.
	MarginMode MarginMode
	Reports    []AccountReport
	Prices     []Price
}

// AccountReport is the normalized state of one Account, or its structured failure.
type AccountReport struct {
	SourceObservedAt                *time.Time
	AccountType                     AccountType
	Status                          string
	ErrorClass                      string
	ErrorDetail                     string
	CollectedAt                     time.Time
	Equity                          *string
	AdjustedEquity                  *string
	InitialMarginRequirementUSD     *string
	MaintenanceMarginRequirementUSD *string
	SpotMarginUnrealizedPnlUSD      *string
	AvailableEquity                 *string
	ReportedInitialMarginRatio      *string
	ReportedMaintenanceMarginRatio  *string
	ReportedMarginRatio             *string
	Balances                        []Balance
	Positions                       []Position
	DualPositions                   []DualPosition
	EarnPositions                   []EarnPosition
}

// Balance is one coin line within an Account Report.
type Balance struct {
	AssetSymbol         string
	Equity              string
	AvailableToWithdraw *string
	AvailableToTrade    *string
	Liability           *string
	Interest            *string
	UnrealizedPnl       *string
}

// FuturesContract describes the tradable contract behind a Position.
type FuturesContract struct {
	Symbol                string     `json:"symbol"`
	Kind                  string     `json:"kind"`
	BaseAssetSymbol       string     `json:"baseAssetSymbol"`
	QuoteAssetSymbol      string     `json:"quoteAssetSymbol"`
	SettlementAssetSymbol string     `json:"settlementAssetSymbol"`
	ExpiryAt              *time.Time `json:"expiryAt"`
	ContractSize          *string    `json:"contractSize"`
}

// Position is one open position within an Account Report.
type Position struct {
	PositionKey        string
	FuturesContract    FuturesContract
	Side               string
	PositionMode       string
	MarginSetting      string
	MarginAssetSymbol  string
	MarginBasis        string
	SourceQuantity     string
	SourceQuantityUnit string
	EntryPrice         *string
	MarkPrice          *string
	LiquidationPrice   *string
	UnrealizedPnl      *string
	Leverage           *string
	Margin             *string
}

// Price is a Platform market price for a Canonical Asset, quoted in another Canonical Asset.
type Price struct {
	CanonicalAssetSymbol      string
	Price                     string
	QuoteCanonicalAssetSymbol string
	RetrievedAt               time.Time
	SourceObservedAt          *time.Time
	SourceMarketRef           string
}
