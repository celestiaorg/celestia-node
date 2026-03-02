package state

import (
	"context"
	"errors"
	"fmt"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	apptypes "github.com/celestiaorg/celestia-app/v7/x/blob/types"
	libshare "github.com/celestiaorg/go-square/v3/share"
)

var ErrGasPriceExceedsLimit = errors.New("state: estimated gas price exceeds max gas price in tx config")

// estimateGasPrice queries the gas price for a transaction via the estimator
// service, unless user specifies a GasPrice via the TxConfig.
func (ca *CoreAccessor) estimateGasPrice(ctx context.Context, cfg *TxConfig) (float64, error) {
	// if user set gas price, use it
	if cfg.GasPrice() != DefaultGasPrice {
		return cfg.GasPrice(), nil
	}

	gasPrice, err := ca.client.EstimateGasPrice(ctx, cfg.priority.toApp())
	if err != nil {
		return 0, err
	}

	// sanity check against max gas price
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
func (ca *CoreAccessor) estimateGasForBlobs(signer string, blobs []*libshare.Blob) (uint64, error) {
	msg, err := apptypes.NewMsgPayForBlobs(signer, 0, blobs...)
	if err != nil {
		return 0, fmt.Errorf("failed to create msg pay-for-blobs: %w", err)
	}
	return apptypes.DefaultEstimateGas(msg), nil
}
