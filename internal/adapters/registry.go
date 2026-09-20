package adapters

import "context"

// CollectionRequest is the input to a Platform adapter.
type CollectionRequest struct {
	Scope string
}

// NormalizedBatch is a sanitized adapter result. Raw Platform payloads never appear here.
type NormalizedBatch struct {
	AdapterName    string
	AdapterVersion string
}

// Adapter collects normalized facts for one Connection.
type Adapter interface {
	Collect(ctx context.Context, connectionID string, req CollectionRequest) (NormalizedBatch, error)
}

// Registry maps Platform codes to adapters. Phase 1 ships empty.
type Registry struct {
	byCode map[string]Adapter
}

// NewRegistry returns a registry with no Platform adapters.
func NewRegistry() *Registry {
	return &Registry{byCode: map[string]Adapter{}}
}

// Lookup returns the adapter for a Platform code.
func (r *Registry) Lookup(platformCode string) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.byCode[platformCode]
	return adapter, ok
}

// Empty reports whether any adapter is registered.
func (r *Registry) Empty() bool {
	return r == nil || len(r.byCode) == 0
}
