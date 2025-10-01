package state

import (
	"fmt"

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
	// TxWorkerAccounts defines how many accounts the TxClient should manage for
	// PayForBlob submissions. Setting the value to a value of 1 enables queued submission,
	// which means blobs are added to a queue and submitted one after another. No additional
	// accounts are initialized. Values greater than 1 enable automatic creation and management
	// of additional worker accounts for parallel submissions. This means that blobs can be
	// submitted by multiple different signers, and that blobs will not be submitted on chain in
	// the original sending order. This is highly recommended for high throughput chains.
	TxWorkerAccounts int
}

func DefaultConfig() Config {
	return Config{
		DefaultKeyName:     DefaultKeyName,
		DefaultBackendName: defaultBackendName,
		EstimatorAddress:   "",
		TxWorkerAccounts:   1,
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if cfg.TxWorkerAccounts < 0 {
		return fmt.Errorf("worker accounts must be zero or positive")
	}

	if cfg.EstimatorAddress == "" {
		return nil
	}

	parsedAddr := utils.NormalizeAddress(cfg.EstimatorAddress)
	cfg.EstimatorAddress = parsedAddr

	return nil
}
