package seeds

import "github.com/google/uuid"

var (
	UserAlice = uuid.MustParse("01a0bdfc-2b00-7001-8000-000000000001")
	UserBob   = uuid.MustParse("01a0bdfc-2b00-7001-8000-000000000002")

	PortfolioAliceMain    = uuid.MustParse("01a0bdfc-2b00-7002-8000-000000000001")
	PortfolioAliceTrading = uuid.MustParse("01a0bdfc-2b00-7002-8000-000000000002")
	PortfolioBobMain      = uuid.MustParse("01a0bdfc-2b00-7002-8000-000000000003")

	IdentityAliceBinance = uuid.MustParse("01a0bdfc-2b00-7003-8000-000000000001")
	IdentityAliceOKX     = uuid.MustParse("01a0bdfc-2b00-7003-8000-000000000002")
	IdentityBobBinance   = uuid.MustParse("01a0bdfc-2b00-7003-8000-000000000003")

	AssetUSDT = uuid.MustParse("01a0bdfc-2b00-7006-8000-000000000001")
	AssetBTC  = uuid.MustParse("01a0bdfc-2b00-7006-8000-000000000002")
)
