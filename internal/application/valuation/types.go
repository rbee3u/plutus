// Package valuation calculates, stores, and rebuilds Valuation Points from Account Reports.
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
// Its symbol JSON tag preserves stored valuations; HTTP uses assetSymbol.
type Omitted struct {
	AssetSymbol string `json:"symbol"`
	// CanonicalAssetSymbol is set only when it differs from AssetSymbol.
	CanonicalAssetSymbol string `json:"assetSymbol,omitempty"`
	Equity               string `json:"equity"`
	Reason               string `json:"reason"`
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
	ManualModifiedAt       *time.Time
	manualState            string
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
	AccountStructure       string  `json:"-"`
	SourceMarginMode       *string `json:"-"`
}

// Completeness returns complete or incomplete for the account.
func (a *AccountValuation) Completeness() string {
	if a.Method == MethodMissing || len(a.Omitted) > 0 {
		return CompletenessIncomplete
	}
	return CompletenessComplete
}
