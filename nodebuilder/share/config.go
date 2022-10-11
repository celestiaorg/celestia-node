package share

import (
	"errors"
	"fmt"
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
	// Availability Timeout Duration
	AvailabilityTimeout time.Duration // TODO(team): should we define a unique timeout for each type of availability?
	// Amount of Samples to perform
	SampleAmount uint
}

func DefaultConfig() Config {
	return Config{
		PeersLimit:          3,
		DiscoveryInterval:   time.Second * 30,
		AdvertiseInterval:   time.Second * 30,
		AvailabilityTimeout: time.Minute * 20,
		SampleAmount:        16,
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if cfg.DiscoveryInterval <= 0 || cfg.AdvertiseInterval <= 0 || cfg.AvailabilityTimeout <= 0 {
		return fmt.Errorf("nodebuilder/share: %s", ErrNegativeInterval)
	}
	return nil
}
