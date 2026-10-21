package adapter

// Platform is a supported exchange code.
type Platform string

// AccountStructure is how a platform arranges an identity's accounts. It decides which account
// types exist, is declared when the identity is created, and never changes afterwards.
type AccountStructure string

// MarginMode is the margin regime a platform applies to a whole trading account within an account
// structure. Plutus observes it from the platform; only some account structures have one.
type MarginMode string

// AccountType is a platform account or the application-owned manual account.
type AccountType string

// TypeManual is managed by Plutus and must never be passed to an adapter.
const TypeManual AccountType = "manual"

// Supported platforms.
const (
	PlatformBinance Platform = "binance"
	PlatformOKX     Platform = "okx"
	PlatformBybit   Platform = "bybit"
	PlatformKraken  Platform = "kraken"
)

// Supported account structures.
const (
	StructureBinanceClassic AccountStructure = "binance_classic"
	StructureBinancePM      AccountStructure = "binance_pm"
	StructureOKXUnified     AccountStructure = "okx_unified"
	StructureBybitUTA2      AccountStructure = "bybit_uta_2_0"
	StructureKrakenSeparate AccountStructure = "kraken_separate"
	StructureKrakenUnified  AccountStructure = "kraken_unified"
)

// Supported margin modes. Binance and Kraken structures have none.
const (
	MarginOKXSpot       MarginMode = "okx_spot"
	MarginOKXFutures    MarginMode = "okx_futures"
	MarginOKXMCM        MarginMode = "okx_mcm"
	MarginOKXPM         MarginMode = "okx_pm"
	MarginBybitIsolated MarginMode = "bybit_isolated"
	MarginBybitCross    MarginMode = "bybit_cross"
	MarginBybitPM       MarginMode = "bybit_pm"
)

// Report statuses.
const (
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// Position sides and quantity units.
const (
	SideLong             = "long"
	SideShort            = "short"
	UnitContracts        = "contracts"
	UnitBase             = "base"
	UnitQuote            = "quote"
	KindLinearPerpetual  = "linear_perpetual"
	KindInversePerpetual = "inverse_perpetual"
	KindLinearDelivery   = "linear_delivery"
	KindInverseDelivery  = "inverse_delivery"
)
