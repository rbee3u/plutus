package adapter

import (
	"time"
)

// ValidationResult is the outcome of one read-only validation.
type ValidationResult struct {
	// ExternalRef is the platform's own reference of the account the key reaches.
	ExternalRef string
	// MarginMode is the margin mode the platform reports, empty for a structure that has none.
	MarginMode MarginMode
}

// Batch is one normalized collection result for one identity.
type Batch struct {
	CollectedAt time.Time
	// MarginMode is the margin mode the platform reported at the start of the run, empty for a
	// structure that has none.
	MarginMode MarginMode
	Reports    []AccountReport
	Prices     []Price
}

// AccountReport is the normalized state of one account, or its structured failure.
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

// Balance is one coin line within an account report.
type Balance struct {
	AssetSymbol         string
	Equity              string
	AvailableToWithdraw *string
	AvailableToTrade    *string
	Liability           *string
	Interest            *string
	UnrealizedPnl       *string
}

// FuturesContract describes the tradable contract behind a position.
type FuturesContract struct {
	Symbol                string     `json:"symbol"`
	Kind                  string     `json:"kind"`
	BaseAssetSymbol       string     `json:"baseAssetSymbol"`
	QuoteAssetSymbol      string     `json:"quoteAssetSymbol"`
	SettlementAssetSymbol string     `json:"settlementAssetSymbol"`
	ExpiryAt              *time.Time `json:"expiryAt"`
	ContractSize          *string    `json:"contractSize"`
}

// Position is one open position within an account report.
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

// Price is a platform market price for a canonical asset, quoted in another canonical asset.
type Price struct {
	CanonicalAssetSymbol      string
	Price                     string
	QuoteCanonicalAssetSymbol string
	RetrievedAt               time.Time
	SourceObservedAt          *time.Time
	SourceMarketRef           string
}
