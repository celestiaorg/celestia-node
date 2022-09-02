package share

import (
	"errors"
	"time"
)

var (
	ErrNegativeInterval = errors.New("interval must be positive")
)

type Config struct {
	// PeersLimit defines how many peers will be added during discovery.
	PeersLimit uint
	// DiscoveryInterval is an interval between discovery sessions.
	DiscoveryInterval time.Duration
	// AdvertiseInterval is a interval between advertising sessions.
	// NOTE: only full and bridge can advertise themselves.
	AdvertiseInterval time.Duration
}

func DefaultConfig() Config {
	return Config{
		PeersLimit:        3,
		DiscoveryInterval: time.Second * 30,
		AdvertiseInterval: time.Second * 30,
	}
}

// ValidateBasic performs basic validation of the config.
func (cfg *Config) ValidateBasic() error {
	if cfg.DiscoveryInterval <= 0 || cfg.AdvertiseInterval <= 0 {
		return ErrNegativeInterval
	}
	return nil
}
