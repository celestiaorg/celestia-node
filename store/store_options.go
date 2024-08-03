package store

type Parameters struct {
	// RecentBlocksCacheSize is the size of the cache for recent blocks. Set 0 to disable this cache.
	RecentBlocksCacheSize int

	// AvailabilityCacheSize is the size of the cache for accessors requested for serving availability
	// samples. Set 0 to disable this cache.
	AvailabilityCacheSize int
}

// DefaultParameters returns the default configuration values for the EDS store parameters.
func DefaultParameters() *Parameters {
	return &Parameters{
		RecentBlocksCacheSize: 10,
		AvailabilityCacheSize: 128,
	}
}

func (p *Parameters) Validate() error {
	return nil
}
