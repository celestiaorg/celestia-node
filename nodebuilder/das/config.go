package das

import (
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-node/das"
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
func DefaultConfig() Config {
	cfg := das.DefaultParameters()
	cfg.SampleTimeout = modp2p.BlockTime * time.Duration(cfg.SampleTimeout)
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
