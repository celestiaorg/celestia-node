package state

import (
	"context"
	"errors"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	apptypes "github.com/celestiaorg/celestia-app/v3/x/blob/types"
)

// gasMultiplier is used to increase gas limit if tx has additional options.
const gasMultiplier = 1.1

var ErrGasPriceExceedsLimit = errors.New("state: estimated gasPrice exceeds max gasPrice")

func (ca *CoreAccessor) estimateGasPrice(ctx context.Context, cfg *TxConfig) (float64, error) {
	gasPrice, err := ca.client.EstimateGasPrice(ctx, cfg.priority.toApp())
	if err != nil {
		return 0, err
	}

	// sanity check estimated gas price against user's set gas price
	if cfg.GasPrice() != DefaultGasPrice {
		gasPrice = cfg.GasPrice()
	}
	if gasPrice > cfg.MaxGasPrice() {
		return 0, ErrGasPriceExceedsLimit
	}

	return gasPrice, nil
}

// estimate the gas price and limit.
// if gas price and gas limit are both set,
// then these two values will be returned, bypassing other checks.
func (ca *CoreAccessor) estimateGasPriceAndUsage(
	ctx context.Context,
	cfg *TxConfig,
	msg sdktypes.Msg,
) (float64, uint64, error) {
	if cfg.GasPrice() != DefaultGasPrice && cfg.GasLimit() != 0 {
		return cfg.GasPrice(), cfg.GasLimit(), nil
	}

	gasPrice, gasLimit, err := ca.client.EstimateGasPriceAndUsage(ctx, []sdktypes.Msg{msg}, cfg.priority.toApp())
	if err != nil {
		return 0, 0, err
	}

	// sanity check estimated gas price against user's set gas price
	if cfg.GasPrice() != DefaultGasPrice {
		gasPrice = cfg.GasPrice()
	}
	if gasPrice > cfg.MaxGasPrice() {
		return 0, 0, ErrGasPriceExceedsLimit
	}

	// sanity check estimated gas usage against user's set gas limit
	if cfg.GasLimit() != 0 {
		gasLimit = cfg.GasLimit()
	}

	return gasPrice, gasLimit, nil
}

// estimateGasForBlobs returns a gas limit that can be applied to the `MsgPayForBlob` transactions.
// NOTE: final result of the estimation will be multiplied by the `gasMultiplier`(1.1)
// to cover additional options of the Tx.
func (ca *CoreAccessor) estimateGasForBlobs(blobSizes []uint32) uint64 {
	gas := apptypes.DefaultEstimateGas(blobSizes)
	return uint64(float64(gas) * gasMultiplier)
}
