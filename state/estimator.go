package state

import (
	"context"
	"errors"
	"fmt"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v3/pkg/user"
	apptypes "github.com/celestiaorg/celestia-app/v3/x/blob/types"
)

// gasMultiplier is used to increase gas limit in case if tx has additional options.
const gasMultiplier = 1.1

type estimator struct {
	estimatorAddress string

	defaultClientConn *grpc.ClientConn
	estimatorConn     *grpc.ClientConn
}

func (e *estimator) connect() {
	if e.estimatorConn != nil && e.estimatorConn.GetState() != connectivity.Shutdown {
		return
	}
	if e.estimatorAddress == "" {
		return
	}

	conn, err := grpc.NewClient(e.estimatorAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Warn("state: failed to connect to estimator endpoint", "err", err)
		return
	}
	e.estimatorConn = conn
}

// estimateGas estimates gas in case it has not been set.
func (e *estimator) estimateGas(
	ctx context.Context,
	client *user.TxClient,
	priority TxPriority,
	msg sdktypes.Msg,
) (float64, uint64, error) {
	signer := client.Signer()
	rawTx, err := signer.CreateTx([]sdktypes.Msg{msg}, user.SetFee(1))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create raw tx: %w", err)
	}

	gasPrice, gas, err := e.queryGasUsedAndPrice(ctx, signer, priority, rawTx)
	if err == nil {
		return gasPrice, uint64(float64(gas) * gasMultiplier), nil
	}

	// Tx client will multiple estimated gas by 1.1 to cover additional costs
	gas, err = client.EstimateGas(ctx, []sdktypes.Msg{msg})
	if err != nil {
		return 0, 0, fmt.Errorf("state: failed to estimate gas: %w", err)
	}
	return DefaultGasPrice, gas, nil
}

func (e *estimator) queryGasUsedAndPrice(
	ctx context.Context,
	signer *user.Signer,
	priority TxPriority,
	rawTx []byte,
) (float64, uint64, error) {
	e.connect()

	if e.estimatorConn != nil && e.estimatorConn.GetState() != connectivity.Shutdown {
		gasPrice, gas, err := signer.QueryGasUsedAndPrice(ctx, e.estimatorConn, priority.toApp(), rawTx)
		if err == nil {
			return gasPrice, gas, nil
		}
		log.Warn("failed to query gas used and price from the estimator endpoint.", "err", err)
	}

	if e.defaultClientConn == nil || e.defaultClientConn.GetState() == connectivity.Shutdown {
		return 0, 0, errors.New("connection with the consensus node is dropped")
	}

	gasPrice, gas, err := signer.QueryGasUsedAndPrice(ctx, e.defaultClientConn, priority.toApp(), rawTx)
	if err != nil {
		log.Warn("state: failed to query gas used and price from the default endpoint", "err", err)
	}
	return gasPrice, gas, err
}

// estimateGasForBlobs returns a gas limit that can be applied to the `MsgPayForBlob` transactions.
// NOTE: final result of the estimation will be multiplied by the `gasMultiplier`(1.1)
// to cover additional options of the Tx.
func (e *estimator) estimateGasForBlobs(blobSizes []uint32) uint64 {
	gas := apptypes.DefaultEstimateGas(blobSizes)
	return uint64(float64(gas) * gasMultiplier)
}
