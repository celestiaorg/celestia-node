package das

import (
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// Config contains configuration parameters for the DASer (or DASing process)
type Config struct {
	das.Parameters
	// Enabled controls whether the DASer is started at all. If false, the DASer is disabled and not invoked.
	Enabled bool `json:"enabled"`
}

// TODO(@derrandz): parameters needs performance testing on real network to define optimal values
// DefaultConfig provide the optimal default configuration per node type.
// For the moment, there is only one default configuration for all node types
// but this function will provide more once #1261 is addressed.
//
// TODO(@derrandz): Address #1261
func DefaultConfig(tp node.Type) Config {
	cfg := Config{
		Parameters: das.DefaultParameters(),
		Enabled:    true, // Enabled by default
	}
	switch tp {
	case node.Light:
		cfg.SampleTimeout = modp2p.BlockTime * time.Duration(cfg.ConcurrencyLimit)
	}
	return cfg
}

// Validate performs basic validation of the config.
// Upon encountering an invalid value, Validate returns an error of type: ErrMisConfig
func (cfg *Config) Validate() error {
	err := cfg.Parameters.Validate()
	if err != nil {
		return fmt.Errorf("moddas misconfiguration: %w", err)
	}

	return nil
}
