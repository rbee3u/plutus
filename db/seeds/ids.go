// Package seeds generates deterministic development data for every seed profile.
package seeds

import (
	"encoding/binary"

	"github.com/google/uuid"
)

// Kind nibbles keep seeded IDs recognisable: 7 is the UUID version, the low byte is the entity kind.
const (
	kindUser        uint16 = 0x7001
	kindPortfolio   uint16 = 0x7002
	kindIdentity    uint16 = 0x7003
	kindAccount     uint16 = 0x7005
	kindListing     uint16 = 0x7007
	kindInstrument  uint16 = 0x7008
	kindJob         uint16 = 0x7010
	kindReport      uint16 = 0x7011
	kindAudit       uint16 = 0x7015
	kindCapitalFlow uint16 = 0x7032
)

// SeedID derives a stable UUID from an entity kind and a sequence number.
func SeedID(kind uint16, n uint64) uuid.UUID {
	var id uuid.UUID
	copy(id[:6], []byte{0x01, 0xA0, 0xBD, 0xFC, 0x2B, 0x00})
	binary.BigEndian.PutUint16(id[6:8], kind)
	id[8] = 0x80
	id[9] = 0x00
	var tail [8]byte
	binary.BigEndian.PutUint64(tail[:], n)
	copy(id[10:16], tail[2:8])
	return id
}

// Well-known IDs shared by profiles, tests, and walkthroughs.
var (
	UserAlice = SeedID(kindUser, 1)
	UserBob   = SeedID(kindUser, 2)

	PortfolioAliceMain    = SeedID(kindPortfolio, 1)
	PortfolioAliceTrading = SeedID(kindPortfolio, 2)
	PortfolioBobMain      = SeedID(kindPortfolio, 3)

	IdentityAliceBinance    = SeedID(kindIdentity, 1)
	IdentityAliceOKX        = SeedID(kindIdentity, 2)
	IdentityAliceBybit      = SeedID(kindIdentity, 3)
	IdentityAliceBinanceOld = SeedID(kindIdentity, 4)
	IdentityBobBinance      = SeedID(kindIdentity, 5)

	AccountAliceBinancePM      = SeedID(kindAccount, 1)
	AccountAliceBinanceSpot    = SeedID(kindAccount, 2)
	AccountAliceBinanceFunding = SeedID(kindAccount, 3)
	AccountAliceBinanceEarn    = SeedID(kindAccount, 4)
	AccountAliceOKXTrading     = SeedID(kindAccount, 5)
	AccountAliceOKXFunding     = SeedID(kindAccount, 6)
	AccountAliceOKXEarn        = SeedID(kindAccount, 7)
	AccountAliceBybitUnified   = SeedID(kindAccount, 8)
	AccountAliceBybitFunding   = SeedID(kindAccount, 9)
	AccountAliceBybitEarn      = SeedID(kindAccount, 10)
	AccountAliceOldPM          = SeedID(kindAccount, 11)
	AccountAliceOldSpot        = SeedID(kindAccount, 12)
	AccountAliceOldFunding     = SeedID(kindAccount, 13)
	AccountAliceOldEarn        = SeedID(kindAccount, 14)
	AccountBobBinancePM        = SeedID(kindAccount, 15)
	AccountBobBinanceSpot      = SeedID(kindAccount, 16)
	AccountBobBinanceFunding   = SeedID(kindAccount, 17)
	AccountBobBinanceEarn      = SeedID(kindAccount, 18)

	FlowAliceBTC     = SeedID(kindCapitalFlow, 1)
	FlowAliceUSDT    = SeedID(kindCapitalFlow, 2)
	FlowAliceTOKENX  = SeedID(kindCapitalFlow, 3)
	FlowAliceUSDC    = SeedID(kindCapitalFlow, 4)
	FlowAliceETH     = SeedID(kindCapitalFlow, 5)
	FlowBobUSDT      = SeedID(kindCapitalFlow, 6)
	FlowAliceBTCOut  = SeedID(kindCapitalFlow, 7)
	FlowAliceUSDTOut = SeedID(kindCapitalFlow, 8)
)
