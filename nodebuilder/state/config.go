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
	// TxWorkerAccounts is used for queued submission. It defines how many accounts the
	// TxClient uses for PayForBlob submissions.
	//   - Value of 0 submits transactions immediately (without a submission queue).
	//   - Value of 1 uses synchronous submission (submission queue with default
	//     signer as author of transactions).
	//   - Value of > 1 uses parallel submission (submission queue with several accounts
	//     submitting blobs). Parallel submission is not guaranteed to include blobs
	//     in the same order as they were submitted.
	TxWorkerAccounts int
}

func DefaultConfig() Config {
	return Config{
		DefaultKeyName:     DefaultKeyName,
		DefaultBackendName: defaultBackendName,
		EstimatorAddress:   "",
		TxWorkerAccounts:   0,
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if cfg.TxWorkerAccounts < 0 {
		return fmt.Errorf("worker accounts must be non-negative")
	}

	if cfg.EstimatorAddress == "" {
		return nil
	}

	parsedAddr := utils.NormalizeAddress(cfg.EstimatorAddress)
	cfg.EstimatorAddress = parsedAddr

	return nil
}
