package pruner

import "time"

type Config struct {
	PruningEnabled bool
	EpochDuration  time.Duration
	RecencyWindow  time.Duration
}

func DefaultConfig() Config {
	return Config{
		PruningEnabled: true,
		EpochDuration:  time.Minute * 1,
		RecencyWindow:  time.Hour,
	}
}

// WithStoragePrunerMetrics is a utility function to turn on storage pruner metrics and that is
// expected to be "invoked" by the fx lifecycle.
func WithStoragePrunerMetrics(sp *StoragePruner) error {
	return sp.WithMetrics()
}
