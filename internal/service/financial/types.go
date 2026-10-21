package financial

import (
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/service/valuation"
)

// Context describes which point a read was answered from.
type Context struct {
	PortfolioID uuid.UUID
	Timezone    string
	CycleID     *uuid.UUID
	EvaluatedAt *time.Time
}

// Overview is the latest point and its ownership tree in one read snapshot.
type Overview struct {
	Context   Context
	Point     *valuation.Point
	Platforms []TreePlatform
	Issues    []valuation.Issue
}

// TreeIdentity groups accounts of one identity.
type TreeIdentity struct {
	ID               uuid.UUID
	DisplayName      string
	Status           string
	ConnectionStatus string
	ValueUSD         *string
	Completeness     string
	Accounts         []valuation.AccountValuation
}

// TreePlatform groups identities of one platform.
type TreePlatform struct {
	Platform     string
	Label        string
	ValueUSD     *string
	Completeness string
	Identities   []TreeIdentity
}

// BalanceRow is one balance with its USD value.
type BalanceRow struct {
	ManualChange         string
	ManualQuantity       *string
	Excluded             bool
	AccountID            uuid.UUID
	AccountName          string
	AccountType          string
	IdentityID           uuid.UUID
	IdentityName         string
	Platform             string
	AssetID              uuid.UUID
	AssetSymbol          string
	CanonicalAssetSymbol string
	Equity               string
	AvailableToWithdraw  *string
	AvailableToTrade     *string
	Liability            *string
	Interest             *string
	UnrealizedPnl        *string
	PriceUSD             *string
	EquityUSD            *string
	NoPrice              bool
	Freshness            string
	ReportObservedAt     *time.Time
	ManualModifiedAt     *time.Time
	equityUSDRat         *big.Rat
}

// PositionRow is one position with its USD notional.
type PositionRow struct {
	AccountID               uuid.UUID
	AccountName             string
	AccountType             string
	IdentityID              uuid.UUID
	IdentityName            string
	Platform                string
	PositionKey             string
	FuturesContractID       uuid.UUID
	FuturesSymbol           string
	Kind                    string
	BaseAssetSymbol         *string
	QuoteAssetSymbol        *string
	SettlementAssetSymbol   *string
	ExpiryAt                *time.Time
	ContractSize            *string
	Side                    string
	PositionMode            *string
	MarginSetting           *string
	MarginAssetSymbol       *string
	MarginBasis             *string
	PortfolioMargin         *bool
	Size                    *string
	SizeSymbol              *string
	SettleMargin            *string
	MarginSource            *string
	MarginUnavailableReason *string
	MarginUnavailableCode   *string
	DisplayLeverage         *string
	Issues                  []valuation.Issue
	SourceQuantity          string
	SourceQuantityUnit      string
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

// AccountView is one account's aligned report with its applicable holdings.
type AccountView struct {
	Context       Context
	Account       valuation.AccountValuation
	Balances      []BalanceRow
	Positions     []PositionRow
	DualPositions []DualPositionRow
	EarnPositions []EarnPositionRow
	Issues        []valuation.Issue
	ManualOverlay *ManualOverlay
}

// ReportCoverage qualifies the account set before symbol search and pagination.
// Compatible stale reports count as readable; optional metadata does not.
type ReportCoverage struct {
	Complete          bool
	MissingAccountIDs []uuid.UUID
}

// ManualOverlay identifies current edits independently of the collected point.
type ManualOverlay struct {
	Versions []ManualLedgerVersion
}

type ManualLedgerVersion struct {
	AccountID uuid.UUID
	Version   uint64
}

// ListFilter selects flat balance or position rows.
type ListFilter struct {
	IncludePendingManual bool
	CycleID              *uuid.UUID
	Platform             string
	IdentityID           *uuid.UUID
	Symbol               string
	Offset               int
	Limit                int
}

// FlatList is an offset page of holdings with totals and completeness.
type FlatList[T any] struct {
	Context           Context
	Items             []T
	Offset            int
	Limit             int
	Total             int
	TotalUSD          *string
	Completeness      string
	Issues            []valuation.Issue
	Coverage          ReportCoverage
	TotalCompleteness string
	TotalFreshness    string
	TotalIssues       []valuation.Issue
	ManualOverlay     *ManualOverlay
}

// PointDetail is one point with its account valuations.
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
	Issues       []valuation.Issue
}

// ValueChange is end − start − contributions + withdrawals over a period.
type ValueChange struct {
	Comparable               bool
	FlowsCompleteness        string
	PeriodKind               string
	Label                    string
	From                     time.Time
	To                       time.Time
	StartPoint               *valuation.Point
	EndPoint                 *valuation.Point
	ContributionsUSD         *string
	WithdrawalsUSD           *string
	BoundaryContributionsUSD *string
	BoundaryWithdrawalsUSD   *string
	AmountUSD                *string
	Completeness             string
	Issues                   []valuation.Issue
	BoundaryEvents           []valuation.BoundaryEvent
}
