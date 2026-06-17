package node

import (
	"fmt"
	"time"
)

var defaultLifecycleTimeout = time.Minute * 2

type Config struct {
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
	// BadgerValueThreshold is the byte size above which Badger datastore values
	// are kept in the value log instead of the LSM tree. 0 = upstream default
	// (share size, 512B), which forces ExtendedHeaders into the append-only value
	// log; on long-running nodes that grows `data/` by GBs/hour because deferred
	// value-log GC cannot reclaim header churn fast enough. Raising it (e.g.
	// 1048576 = 1 MiB) keeps headers in the LSM tree, where leveled compaction
	// reclaims overwrites continuously and `data/` stays bounded — at the cost of
	// more memory during compaction. See nodebuilder/store.go constraintBadgerConfig.
	BadgerValueThreshold int `toml:",omitempty"`
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
