// Package adapters defines the read-only platform contract and product catalog.
package adapters

import (
	"context"
)

// Adapter collects normalized facts through read-only endpoints. Implementations
// check account structure and margin mode where platform access permits;
// detected mismatches and unknown modes fail with ClassUnsupported.
type Adapter interface {
	// Validate checks the key and returns the platform's own account reference and margin mode.
	Validate(ctx context.Context, creds Credentials, structure AccountStructure) (ValidationResult, error)
	// Collect returns one report per requested account and associated market prices.
	Collect(ctx context.Context, creds Credentials, structure AccountStructure, accounts []AccountType) (Batch, error)
}

// MarketData reads normalized public prices independently of private credentials.
// Manual collection supplies canonical asset symbols from immutable manual revisions.
type MarketData interface {
	CollectPrices(context.Context, []string) ([]Price, error)
}
