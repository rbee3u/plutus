package adapter

// Registry maps platform codes to adapters.
type Registry struct {
	byPlatform map[Platform]Adapter
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byPlatform: map[Platform]Adapter{}}
}

// Register installs an adapter for a platform.
func (r *Registry) Register(platform Platform, adapter Adapter) {
	if r == nil || adapter == nil {
		return
	}
	r.byPlatform[platform] = adapter
}

// Lookup returns the adapter for a platform.
func (r *Registry) Lookup(platform Platform) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.byPlatform[platform]
	return adapter, ok
}

// Available reports whether an adapter exists for the platform.
func (r *Registry) Available(platform Platform) bool {
	_, ok := r.Lookup(platform)
	return ok
}
