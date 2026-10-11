// Package valuation calculates, and stores immutable Valuation Points from Account Reports.
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
	AccountName string     `json:"-"`
	// AssetID is stored; Symbol is its display form.
	AssetID *uuid.UUID `json:"-"`
	Symbol  string     `json:"symbol,omitempty"`
}

// Omitted names one Balance excluded from an Account Valuation.
type Omitted struct {
	AssetID     uuid.UUID `json:"-"`
	AssetSymbol string    `json:"assetSymbol"`
	// CanonicalAssetSymbol is set only when it differs from AssetSymbol.
	CanonicalAssetSymbol string `json:"canonicalAssetSymbol,omitempty"`
	Equity               string `json:"equity"`
	Reason               string `json:"reason"`
}

// BoundaryEvent is an Account entering or leaving the Portfolio Boundary.
type BoundaryEvent struct {
	CycleID           uuid.UUID `json:"-"`
	EvaluatedAt       time.Time `json:"-"`
	AccountID         uuid.UUID `json:"accountId"`
	AccountName       string    `json:"accountName"`
	IdentityID        uuid.UUID `json:"identityId"`
	IdentityName      string    `json:"identityName"`
	Platform          string    `json:"platform"`
	Direction         string    `json:"direction"`
	ValueUSD          *string   `json:"valueUsd"`
	Method            string    `json:"method"`
	ValuationComplete bool      `json:"valuationComplete"`
	valueRat          *big.Rat
}

// Point is one stored Valuation Point.
type Point struct {
	CalculationVersion     uint64
	PortfolioID            uuid.UUID
	CycleID                uuid.UUID
	EvaluatedAt            time.Time
	TotalUSD               string
	AccountCount           int
	StaleAccountCount      int
	IncompleteAccountCount int
	// Boundary totals are derived from the point's Boundary Events on read.
	BoundaryContributionUSD string
	BoundaryWithdrawalUSD   string
	BoundaryEvents          []BoundaryEvent
	Issues                  []Issue
	ManualBalanceChanges    []ManualBalanceChange
	DetailState             string
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
	BalanceValuations      []BalanceValuation
	ManualModifiedAt       *time.Time
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
	MarginRatio            *string
	Omitted                []Omitted
	valueRat               *big.Rat
	AccountStructure       string  `json:"-"`
	SourceMarginMode       *string `json:"-"`
}

// Completeness returns complete or incomplete for the account.
func (a *AccountValuation) Completeness() string {
	for _, line := range a.BalanceValuations {
		if line.Treatment == "included" && line.MissingReason != "" {
			return CompletenessIncomplete
		}
	}
	if a.Method == MethodMissing || len(a.Omitted) > 0 {
		return CompletenessIncomplete
	}
	return CompletenessComplete
}

// CalculationVersion changes only when valuation policy or arithmetic changes.
const CalculationVersion uint64 = 1

// BalanceValuation records decisions without copying quantities or price payloads.
type BalanceValuation struct {
	AssetID              uuid.UUID  `json:"assetId"`
	CanonicalAssetSymbol string     `json:"canonicalAssetSymbol"`
	Treatment            string     `json:"treatment"`
	PricingMethod        string     `json:"pricingMethod"`
	PriceObservationID   *uuid.UUID `json:"priceObservationId,omitempty"`
	ValueUSD             *string    `json:"valueUsd,omitempty"`
	MissingReason        string     `json:"missingReason,omitempty"`
}

// Revision identifies a committed result, independently of its algorithm version.
func (p *Point) Revision() string {
	return p.CycleID.String()
}

// ManualBalanceChange identifies an effective quantity change within an existing
// Identity. It is frozen at first commit, independently of issue text.
type ManualBalanceChange struct {
	AccountID   uuid.UUID
	AccountName string
}

func (a ManualBalanceChange) Issue() Issue {
	return Issue{
		Code: "manual_balance_changed", AccountID: &a.AccountID,
		AccountName: a.AccountName,
		Detail:      "Manual quantities changed; value changes across this point include a user adjustment.",
	}
}
