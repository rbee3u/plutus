// Package valuation writes Valuation Points from Account Reports and answers every financial read.
package valuation

import (
	"math/big"
	"time"

	"github.com/google/uuid"
)

// Valuation methods and freshness values.
const (
	MethodReported   = "reported"
	MethodCalculated = "calculated"
	MethodMissing    = "missing"

	FreshnessFresh = "fresh"
	FreshnessStale = "stale"

	CompletenessComplete   = "complete"
	CompletenessIncomplete = "incomplete"

	DirectionContribution = "contribution"
	DirectionWithdrawal   = "withdrawal"

	// maxPriceAge bounds how old a stored price may be relative to the report it values.
	maxPriceAge = 24 * time.Hour
)

// Issue names one reason a value is stale or incomplete.
type Issue struct {
	Code        string     `json:"code"`
	Detail      string     `json:"detail"`
	AccountID   *uuid.UUID `json:"accountId,omitempty"`
	AccountName string     `json:"accountName,omitempty"`
	Symbol      string     `json:"symbol,omitempty"`
}

// Omitted names one Balance excluded from an Account Valuation.
type Omitted struct {
	Symbol string `json:"symbol"`
	// AssetSymbol is set only when the listing is mapped to a different Asset symbol.
	AssetSymbol string `json:"assetSymbol,omitempty"`
	Equity      string `json:"equity"`
	Reason      string `json:"reason"`
}

// BoundaryEvent is an Account entering or leaving the Portfolio Boundary.
type BoundaryEvent struct {
	PointID      uuid.UUID `json:"-"`
	EvaluatedAt  time.Time `json:"-"`
	AccountID    uuid.UUID `json:"accountId"`
	AccountName  string    `json:"accountName"`
	IdentityID   uuid.UUID `json:"identityId"`
	IdentityName string    `json:"identityName"`
	Platform     string    `json:"platform"`
	Direction    string    `json:"direction"`
	ValueUSD     *string   `json:"valueUsd"`
	Method       string    `json:"method"`
	valueRat     *big.Rat
}

// Point is one stored Valuation Point.
type Point struct {
	ID                      uuid.UUID
	PortfolioID             uuid.UUID
	JobID                   uuid.UUID
	EvaluatedAt             time.Time
	TotalUSD                string
	AccountCount            int
	StaleAccountCount       int
	IncompleteAccountCount  int
	BoundaryContributionUSD string
	BoundaryWithdrawalUSD   string
	BoundaryEvents          []BoundaryEvent
	Issues                  []Issue
	RecalculatedAt          *time.Time
	CreatedAt               time.Time
}

// Completeness returns complete or incomplete for the point.
func (p *Point) Completeness() string {
	if p.IncompleteAccountCount > 0 {
		return CompletenessIncomplete
	}
	return CompletenessComplete
}

// AccountValuation is one Account's report and value at a point.
type AccountValuation struct {
	ScopeVersion           uint16
	ReportID               uuid.UUID
	AccountID              uuid.UUID
	AccountName            string
	AccountType            string
	Selected               bool
	IdentityID             uuid.UUID
	IdentityName           string
	IdentityStatus         string
	ConnectionStatus       string
	Platform               string
	Status                 string
	ErrorClass             *string
	ErrorDetail            *string
	Method                 string
	Freshness              string
	ValueUSD               *string
	ReportObservedAt       *time.Time
	SourceReportID         *uuid.UUID
	Equity                 *string
	AdjustedEquity         *string
	AvailableEquity        *string
	InitialMarginRatio     *string
	MaintenanceMarginRatio *string
	UnifiedMarginRatio     *string
	Omitted                []Omitted
	valueRat               *big.Rat
	accountStructure       string
	sourceMarginMode       *string
}

// Completeness returns complete or incomplete for the account.
func (a *AccountValuation) Completeness() string {
	if a.Method == MethodMissing || len(a.Omitted) > 0 {
		return CompletenessIncomplete
	}
	return CompletenessComplete
}

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
	Point                  *Point
	TotalUSD               *string
	Completeness           string
	AccountCount           int
	StaleAccountCount      int
	IncompleteAccountCount int
	ByPlatform             []GroupTotal
	ByIdentity             []GroupTotal
	Issues                 []Issue
	Sparkline              []Point
}

// TreeAccount is one Account in the holdings tree.
type TreeAccount struct {
	AccountValuation
}

// TreeIdentity groups Accounts of one Identity.
type TreeIdentity struct {
	ID               uuid.UUID
	DisplayName      string
	Status           string
	ConnectionStatus string
	ValueUSD         *string
	Completeness     string
	Accounts         []TreeAccount
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
	Issues       []Issue
}

// BalanceRow is one Balance with its USD value.
type BalanceRow struct {
	AccountID           uuid.UUID
	AccountName         string
	AccountType         string
	IdentityID          uuid.UUID
	IdentityName        string
	Platform            string
	ListingID           uuid.UUID
	Symbol              string
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
	InstrumentID            uuid.UUID
	Symbol                  string
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
	Issues                  []Issue
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
	Context       Context
	Account       AccountValuation
	Balances      []BalanceRow
	Positions     []PositionRow
	DualPositions []DualPositionRow
	Issues        []Issue
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
	Issues       []Issue
}

// PointDetail is one point with its Account Valuations.
type PointDetail struct {
	Point    Point
	Accounts []AccountValuation
}

// PeriodClose is the last point before a local period boundary.
type PeriodClose struct {
	PeriodKind   string
	Label        string
	PeriodStart  time.Time
	PeriodEnd    time.Time
	Point        *Point
	Completeness string
	Reasons      []string
}

// ValueChange is end − start − contributions + withdrawals over a period.
type ValueChange struct {
	PeriodKind               string
	Label                    string
	From                     time.Time
	To                       time.Time
	StartPoint               *Point
	EndPoint                 *Point
	ContributionsUSD         string
	WithdrawalsUSD           string
	BoundaryContributionsUSD string
	BoundaryWithdrawalsUSD   string
	AmountUSD                *string
	Completeness             string
	Reasons                  []string
	BoundaryEvents           []BoundaryEvent
}
