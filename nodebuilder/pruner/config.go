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
		EpochDuration:  time.Minute * 10,
		RecencyWindow:  time.Hour * 24 * 30,
	}
}
