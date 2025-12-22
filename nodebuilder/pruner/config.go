package pruner

import (
	"time"

	"github.com/celestiaorg/celestia-node/share/availability"
)

var MetricsEnabled bool

type Config struct {
	EnableService bool
	StorageWindow time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		EnableService: true,
		StorageWindow: availability.StorageWindow,
	}
}
