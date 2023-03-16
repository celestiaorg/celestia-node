package das

import (
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// Config contains configuration parameters for the DASer (or DASing process)
type Config das.Parameters

// TODO(@derrandz): parameters needs performance testing on real network to define optimal values
// DefaultConfig provide the optimal default configuration per node type.
// For the moment, there is only one default configuration for all node types
// but this function will provide more once #1261 is addressed.
//
// TODO(@derrandz): Address #1261
func DefaultConfig(tp node.Type) Config {
	cfg := das.DefaultParameters()
	switch tp {
	case node.Light:
		cfg.SampleTimeout = modp2p.BlockTime * time.Duration(cfg.ConcurrencyLimit)
	case node.Full:
		// Default value for DASer concurrency limit is based on dasing using ipld getter.
		// Full node will primarily use shrex protocol for sampling, that is much more efficient and can
		// fully utilize nodes bandwidth with lower amount of parallel sampling workers
		cfg.ConcurrencyLimit = 6
		// Full node uses shrex with fallback to ipld to sample, so need 2x amount of time in worst case scenario
		cfg.SampleTimeout = 2 * modp2p.BlockTime * time.Duration(cfg.ConcurrencyLimit)
	}
	return Config(cfg)
}

// Validate performs basic validation of the config.
// Upon encountering an invalid value, Validate returns an error of type: ErrMisConfig
func (cfg *Config) Validate() error {
	err := (*das.Parameters)(cfg).Validate()
	if err != nil {
		return fmt.Errorf("moddas misconfiguration: %w", err)
	}

	return nil
}
