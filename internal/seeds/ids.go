// Package seeds generates deterministic development walkthrough data.
package seeds

import (
	"encoding/binary"

	"github.com/google/uuid"
)

// Kind nibbles keep seeded IDs recognisable: 7 is the UUID version, the low byte is the entity kind.
const (
	kindUser            uint16 = 0x7001
	kindPortfolio       uint16 = 0x7002
	kindIdentity        uint16 = 0x7003
	kindAccount         uint16 = 0x7005
	kindAsset           uint16 = 0x7007
	kindFuturesContract uint16 = 0x7008
	kindCycle           uint16 = 0x7010
	kindReport          uint16 = 0x7011
	kindAudit           uint16 = 0x7015
	kindCapitalFlow     uint16 = 0x7032
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

// Well-known IDs retained across disposable development resets.
var (
	UserAlice           = SeedID(kindUser, 1)
	PortfolioAliceMain  = SeedID(kindPortfolio, 1)
	PortfolioAliceEdge  = namedID("portfolio/edge-cases")
	PortfolioAliceLarge = namedID("portfolio/large")
	PortfolioAliceEmpty = namedID("portfolio/empty")
)

func namedID(key string) uuid.UUID {
	// The fixed namespace keeps names stable when other scenarios are added.
	return uuid.NewSHA1(SeedID(kindUser, 1), []byte(key))
}

func portfolioID(name string) uuid.UUID {
	if name == "default" {
		return PortfolioAliceMain
	}
	return namedID("portfolio/" + name)
}

func identityID(name string) uuid.UUID { return namedID("identity/" + name) }
func accountID(identity string, kind string) uuid.UUID {
	return namedID("account/" + identity + "/" + kind)
}
