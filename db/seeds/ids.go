package seeds

import "github.com/google/uuid"

const (
	kindUser             = 0x7001
	kindPortfolio        = 0x7002
	kindIdentity         = 0x7003
	kindConnection       = 0x7004
	kindAccount          = 0x7005
	kindAsset            = 0x7006
	kindListing          = 0x7007
	kindInstrument       = 0x7008
	kindMapping          = 0x7009
	kindMappingRevision  = 0x700A
	kindPin              = 0x700B
	kindPinRevision      = 0x700C
	kindJob              = 0x7010
	kindRun              = 0x7011
	kindSnapshot         = 0x7012
	kindFailure          = 0x7013
	kindAttempt          = 0x7014
	kindAudit            = 0x7015
	kindObservation      = 0x7020
	kindCapitalBaseline  = 0x7030
	kindBaselineRevision = 0x7031
	kindCapitalFlow      = 0x7032
	kindFlowRevision     = 0x7033
)

var (
	UserAlice = SeedID(kindUser, 1)
	UserBob   = SeedID(kindUser, 2)

	PortfolioAliceMain    = SeedID(kindPortfolio, 1)
	PortfolioAliceTrading = SeedID(kindPortfolio, 2)
	PortfolioBobMain      = SeedID(kindPortfolio, 3)

	IdentityAliceBinance    = SeedID(kindIdentity, 1)
	IdentityAliceOKX        = SeedID(kindIdentity, 2)
	IdentityBobBinance      = SeedID(kindIdentity, 3)
	IdentityAliceBinanceSub = SeedID(kindIdentity, 4)
	IdentityAliceBybit      = SeedID(kindIdentity, 5)
	IdentityAliceOKXPM      = SeedID(kindIdentity, 6)

	ConnectionAliceBinance    = SeedID(kindConnection, 1)
	ConnectionAliceOKX        = SeedID(kindConnection, 2)
	ConnectionBobBinance      = SeedID(kindConnection, 3)
	ConnectionAliceBinanceSub = SeedID(kindConnection, 4)
	ConnectionAliceBybit      = SeedID(kindConnection, 5)

	AccountAliceUnified      = SeedID(kindAccount, 1)
	AccountAliceFunding      = SeedID(kindAccount, 2)
	AccountAliceOKXTrading   = SeedID(kindAccount, 3)
	AccountBobUnified        = SeedID(kindAccount, 4)
	AccountAliceEarn         = SeedID(kindAccount, 5)
	AccountAliceSubUnified   = SeedID(kindAccount, 6)
	AccountAliceBybitUnified = SeedID(kindAccount, 7)
	AccountAliceOKXFunding   = SeedID(kindAccount, 8)
	AccountAliceDisappeared  = SeedID(kindAccount, 9)

	AssetUSDT   = SeedID(kindAsset, 1)
	AssetBTC    = SeedID(kindAsset, 2)
	AssetTOKENX = SeedID(kindAsset, 3)
	AssetETH    = SeedID(kindAsset, 4)
	AssetSOL    = SeedID(kindAsset, 5)
	AssetUSDC   = SeedID(kindAsset, 6)
	AssetWBTC   = SeedID(kindAsset, 7)

	ListingBinanceUSDT    = SeedID(kindListing, 1)
	ListingBinanceBTC     = SeedID(kindListing, 2)
	ListingBinanceUSD     = SeedID(kindListing, 3)
	ListingBinanceTOKENX  = SeedID(kindListing, 4)
	ListingBinanceETH     = SeedID(kindListing, 5)
	ListingBinanceSOL     = SeedID(kindListing, 6)
	ListingBinanceUSDC    = SeedID(kindListing, 7)
	ListingBinanceCNY     = SeedID(kindListing, 8)
	ListingBinanceWBTC    = SeedID(kindListing, 9)
	ListingBinanceUnknown = SeedID(kindListing, 10)
	ListingOKXUSDT        = SeedID(kindListing, 11)
	ListingOKXBTC         = SeedID(kindListing, 12)
	ListingOKXETH         = SeedID(kindListing, 13)
	ListingOKXUSD         = SeedID(kindListing, 14)
	ListingOKXCNY         = SeedID(kindListing, 15)
	ListingBybitUSDT      = SeedID(kindListing, 16)
	ListingBybitBTC       = SeedID(kindListing, 17)
	ListingBybitETH       = SeedID(kindListing, 18)
	ListingBybitUSD       = SeedID(kindListing, 19)
	ListingBybitTOKENX    = SeedID(kindListing, 20)

	InstrumentBinanceBTCUSDT     = SeedID(kindInstrument, 1)
	InstrumentBinanceETHUSDT     = SeedID(kindInstrument, 2)
	InstrumentBinanceBTCUSDInv   = SeedID(kindInstrument, 3)
	InstrumentBinanceBTCDelivery = SeedID(kindInstrument, 4)
	InstrumentBinanceBTCSpot     = SeedID(kindInstrument, 5)
	InstrumentOKXBTCUSDT         = SeedID(kindInstrument, 6)
	InstrumentOKXETHUSDT         = SeedID(kindInstrument, 7)
	InstrumentBybitBTCUSDT       = SeedID(kindInstrument, 8)
)

// SeedID returns the deterministic source-fact UUID for kind and sequence n.
func SeedID(kind uint16, n uint64) uuid.UUID {
	var id uuid.UUID
	id[0] = 0x01
	id[1] = 0xa0
	id[2] = 0xbd
	id[3] = 0xfc
	id[4] = 0x2b
	id[5] = 0x00
	id[6] = byte(kind >> 8)
	id[7] = byte(kind)
	id[8] = 0x80
	id[9] = 0x00
	id[10] = byte(n >> 40)
	id[11] = byte(n >> 32)
	id[12] = byte(n >> 24)
	id[13] = byte(n >> 16)
	id[14] = byte(n >> 8)
	id[15] = byte(n)
	return id
}
