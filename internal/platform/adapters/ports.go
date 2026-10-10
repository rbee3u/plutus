// Package adapters defines the read-only Platform contract and product catalog.
package adapters

import (
	"context"
)

// Adapter collects normalized facts through read-only endpoints. Implementations
// check Account Structure and Margin Mode where Platform access permits;
// detected mismatches and unknown modes fail with ClassUnsupported.
type Adapter interface {
	// Validate checks the key and returns the Platform's own account reference and Margin Mode.
	Validate(ctx context.Context, creds Credentials, structure AccountStructure) (ValidationResult, error)
	// Collect returns one report per requested Account and associated market prices.
	Collect(ctx context.Context, creds Credentials, structure AccountStructure, accounts []AccountType) (Batch, error)
}

// MarketData reads normalized public prices independently of private credentials.
// Manual collection supplies Canonical Asset Symbols from immutable Manual revisions.
type MarketData interface {
	CollectPrices(context.Context, []string) ([]Price, error)
}
