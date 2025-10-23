package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

var defaultBackendName = keyring.BackendTest

// Config contains configuration parameters for constructing
// the node's keyring signer.
type Config struct {
	DefaultKeyName     string
	DefaultBackendName string
	// EstimatorAddress specifies a third-party endpoint that will be used to
	// calculate gas price and gas usage
	EstimatorAddress string
	// EnableEstimatorTLS specifies whether to use TLS for the gRPC connection to the
	// estimator service
	EnableEstimatorTLS bool
}

func DefaultConfig() Config {
	return Config{
		DefaultKeyName:     DefaultKeyName,
		DefaultBackendName: defaultBackendName,
		EstimatorAddress:   "",
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if cfg.EstimatorAddress == "" {
		return nil
	}

	parsedAddr := utils.NormalizeAddress(cfg.EstimatorAddress)
	cfg.EstimatorAddress = parsedAddr

	return nil
}
