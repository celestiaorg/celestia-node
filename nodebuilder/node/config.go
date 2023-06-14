package node

import (
	"fmt"
	"time"
)

var defaultLifecycleTimeout = time.Minute * 2

type Config struct {
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// DefaultConfig returns the default node configuration for a given node type.
func DefaultConfig(tp Type) Config {
	var timeout time.Duration
	switch tp {
	case Light:
		timeout = time.Second * 20
	default:
		timeout = defaultLifecycleTimeout
	}
	return Config{
		StartupTimeout:  timeout,
		ShutdownTimeout: timeout,
	}
}

func (c *Config) Validate() error {
	if c.StartupTimeout == 0 {
		return fmt.Errorf("invalid startup timeout: %v", c.StartupTimeout)
	}
	if c.ShutdownTimeout == 0 {
		return fmt.Errorf("invalid shutdown timeout: %v", c.ShutdownTimeout)
	}
	return nil
}
