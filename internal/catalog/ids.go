package catalog

import "github.com/google/uuid"

var (
	// AssetUSD is the seeded USD asset identifier.
	AssetUSD = uuid.MustParse("01a0bdfc-1fa4-7a54-9fa1-24f4721a71af")
	// AssetCNY is the seeded CNY asset identifier.
	AssetCNY = uuid.MustParse("01a0bdfc-1fa4-7af4-bedf-65d968b9122b")
	// PlatformBinance is the seeded Binance platform identifier.
	PlatformBinance = uuid.MustParse("01a0bdfc-1fa4-7b00-b990-a610a553e10c")
	// PlatformOKX is the seeded OKX platform identifier.
	PlatformOKX = uuid.MustParse("01a0bdfc-1fa4-7b08-bc44-a2a59e4665b8")
	// PlatformBybit is the seeded Bybit platform identifier.
	PlatformBybit = uuid.MustParse("01a0bdfc-1fa4-7b10-990c-04aa9d8ac659")
)

const (
	// AccountModeBinancePM is Binance Portfolio Margin.
	AccountModeBinancePM = "binance_pm"
	// AccountModeOKXMCM is OKX Multi-currency margin.
	AccountModeOKXMCM = "okx_mcm"
	// AccountModeOKXPM is OKX Portfolio margin.
	AccountModeOKXPM = "okx_pm"
	// AccountModeBybitUTA20 is Bybit Unified Trading Account 2.0.
	AccountModeBybitUTA20 = "bybit_uta_2_0"
)

// SupportedAccountModes are the ADR 0005 account modes accepted by Identity configuration.
func SupportedAccountModes() []string {
	return []string{AccountModeBinancePM, AccountModeOKXMCM, AccountModeOKXPM, AccountModeBybitUTA20}
}

// AccountModeSupported reports whether mode is an accepted account mode.
func AccountModeSupported(mode string) bool {
	for _, supported := range SupportedAccountModes() {
		if mode == supported {
			return true
		}
	}
	return false
}
