package adapters

// Registry maps Platform codes to adapters.
type Registry struct {
	byPlatform map[Platform]Adapter
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byPlatform: map[Platform]Adapter{}}
}

// Register installs an adapter for a Platform.
func (r *Registry) Register(platform Platform, adapter Adapter) {
	if r == nil || adapter == nil {
		return
	}
	r.byPlatform[platform] = adapter
}

// Lookup returns the adapter for a Platform.
func (r *Registry) Lookup(platform Platform) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.byPlatform[platform]
	return adapter, ok
}

// Available reports whether an adapter exists for the Platform.
func (r *Registry) Available(platform Platform) bool {
	_, ok := r.Lookup(platform)
	return ok
}
