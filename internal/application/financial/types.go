package financial

import (
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
)

// Context describes which point a read was answered from.
type Context struct {
	PortfolioID uuid.UUID
	Timezone    string
	PointID     *uuid.UUID
	JobID       *uuid.UUID
	EvaluatedAt *time.Time
}

// GroupTotal is a backend-computed subtotal.
type GroupTotal struct {
	Key          string
	Label        string
	Platform     string
	ValueUSD     *string
	AccountCount int
	RowCount     int
	Completeness string
}

// Overview is the landing read model.
type Overview struct {
	Context                Context
	Point                  *valuation.Point
	TotalUSD               *string
	Completeness           string
	AccountCount           int
	StaleAccountCount      int
	IncompleteAccountCount int
	ByPlatform             []GroupTotal
	ByIdentity             []GroupTotal
	Issues                 []valuation.Issue
	Sparkline              []valuation.Point
}

// TreeIdentity groups Accounts of one Identity.
type TreeIdentity struct {
	ID               uuid.UUID
	DisplayName      string
	Status           string
	ConnectionStatus string
	ValueUSD         *string
	Completeness     string
	Accounts         []valuation.AccountValuation
}

// TreePlatform groups Identities of one Platform.
type TreePlatform struct {
	Platform     string
	Label        string
	ValueUSD     *string
	Completeness string
	Identities   []TreeIdentity
}

// Tree is the Platform → Identity → Account read model.
type Tree struct {
	Context      Context
	TotalUSD     *string
	Completeness string
	Platforms    []TreePlatform
	Issues       []valuation.Issue
}

// BalanceRow is one Balance with its USD value.
type BalanceRow struct {
	ManualChange        string
	ManualQuantity      *string
	Excluded            bool
	AccountID           uuid.UUID
	AccountName         string
	AccountType         string
	IdentityID          uuid.UUID
	IdentityName        string
	Platform            string
	PlatformAssetID     uuid.UUID
	PlatformAssetSymbol string
	AssetSymbol         string
	Equity              string
	AvailableToWithdraw *string
	AvailableToTrade    *string
	Liability           *string
	Interest            *string
	UnrealizedPnl       *string
	PriceUSD            *string
	EquityUSD           *string
	NoPrice             bool
	Freshness           string
	ReportObservedAt    *time.Time
	ManualModifiedAt    *time.Time
	equityUSDRat        *big.Rat
}

// PositionRow is one Position with its USD notional.
type PositionRow struct {
	AccountID               uuid.UUID
	AccountName             string
	AccountType             string
	IdentityID              uuid.UUID
	IdentityName            string
	Platform                string
	PlatformPositionRef     string
	FuturesContractID       uuid.UUID
	FuturesSymbol           string
	Kind                    string
	BaseSymbol              *string
	QuoteSymbol             *string
	SettlementSymbol        *string
	ExpiryAt                *time.Time
	ContractSize            *string
	Side                    string
	PositionMode            *string
	MarginMode              *string
	MarginCurrency          *string
	MarginBasis             *string
	PortfolioMargin         *bool
	Size                    *string
	SizeSymbol              *string
	SettleMargin            *string
	MarginSource            *string
	MarginUnavailableReason *string
	Issues                  []valuation.Issue
	Quantity                string
	QuantityUnit            string
	EntryPrice              *string
	MarkPrice               *string
	LiquidationPrice        *string
	UnrealizedPnl           *string
	Leverage                *string
	Margin                  *string
	NotionalUSD             *string
	Freshness               string
	ReportObservedAt        *time.Time
}

// AccountView is one Account's current report with its Balances or Positions.
type AccountView struct {
	Context              Context
	Account              valuation.AccountValuation
	Balances             []BalanceRow
	Positions            []PositionRow
	DualPositions        []DualPositionRow
	EarnPositions        []EarnPositionRow
	EarnBalanceSummaries []EarnBalanceSummary
	EarnDetailsCollected bool
	Issues               []valuation.Issue
}

// ListFilter selects flat Balance or Position rows.
type ListFilter struct {
	Platform   string
	IdentityID *uuid.UUID
	Symbol     string
	Offset     int
	Limit      int
}

// FlatList is an offset page of holdings with totals and completeness.
type FlatList[T any] struct {
	Context      Context
	Items        []T
	Offset       int
	Limit        int
	Total        int
	TotalUSD     *string
	Completeness string
	Issues       []valuation.Issue
}

// PointDetail is one point with its Account Valuations.
type PointDetail struct {
	Point    valuation.Point
	Accounts []valuation.AccountValuation
}

// PeriodClose holds the last point within a local calendar period, or nil.
type PeriodClose struct {
	PeriodKind   string
	Label        string
	PeriodStart  time.Time
	PeriodEnd    time.Time
	Point        *valuation.Point
	Completeness string
	Reasons      []string
}

// ValueChange is end − start − contributions + withdrawals over a period.
type ValueChange struct {
	PeriodKind               string
	Label                    string
	From                     time.Time
	To                       time.Time
	StartPoint               *valuation.Point
	EndPoint                 *valuation.Point
	ContributionsUSD         string
	WithdrawalsUSD           string
	BoundaryContributionsUSD string
	BoundaryWithdrawalsUSD   string
	AmountUSD                *string
	Completeness             string
	Reasons                  []string
	BoundaryEvents           []valuation.BoundaryEvent
}
