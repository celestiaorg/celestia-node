package state

import (
	"context"
	"errors"
	"fmt"
	"time"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v3/pkg/user"
	apptypes "github.com/celestiaorg/celestia-app/v3/x/blob/types"
)

// gasMultiplier is used to increase gas limit in case if tx has additional options.
const gasMultiplier = 1.1

var ErrGasPriceExceedsLimit = errors.New("state: estimated gasPrice exceeds max gasPrice")

type estimator struct {
	estimatorAddress string

	defaultClientConn *grpc.ClientConn
	estimatorConn     *grpc.ClientConn
}

func (e *estimator) Start(context.Context) error {
	if e.estimatorAddress == "" {
		return nil
	}

	conn, err := grpc.NewClient(
		e.estimatorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpcRetry()),
	)
	if err != nil {
		return err
	}
	e.estimatorConn = conn
	return nil
}

func (e *estimator) Stop(context.Context) error {
	if e.estimatorConn != nil {
		if err := e.estimatorConn.Close(); err != nil {
			return err
		}
	}
	e.estimatorConn = nil
	return nil
}

// estimate the gas price and limit.
// if gas price and gas limit are both set,
// then these two values will be returned, bypassing other checks.
func (e *estimator) estimate(
	ctx context.Context,
	cfg *TxConfig,
	client *user.TxClient,
	msg sdktypes.Msg,
) (float64, uint64, error) {
	if cfg.GasPrice() != DefaultGasPrice && cfg.GasLimit() != 0 {
		return cfg.GasPrice(), cfg.GasLimit(), nil
	}

	gasPrice, gasLimit, err := e.estimateGas(ctx, client, cfg.priority, msg)
	if err != nil {
		return 0, 0, err
	}

	if cfg.GasPrice() != DefaultGasPrice {
		gasPrice = cfg.GasPrice()
	}
	if cfg.GasLimit() != 0 {
		gasLimit = cfg.GasLimit()
	}
	if gasPrice > cfg.MaxGasPrice() {
		return 0, 0, ErrGasPriceExceedsLimit
	}
	return gasPrice, gasLimit, nil
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
) (gasPrice float64, gas uint64, err error) {
	conns := []*grpc.ClientConn{e.estimatorConn, e.defaultClientConn}
	for _, conn := range conns {
		if conn == nil {
			continue
		}

		gasPrice, gas, err = signer.QueryGasUsedAndPrice(ctx, conn, priority.toApp(), rawTx)
		if err == nil {
			return gasPrice, gas, nil
		}
		log.Warnf("failed to query gas used and price from the endpoint(%s): %v", conn.Target(), err)
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

func grpcRetry() grpc.UnaryClientInterceptor {
	return grpc_retry.UnaryClientInterceptor(
		grpc_retry.WithMax(5),
		grpc_retry.WithCodes(codes.Unavailable),
		grpc_retry.WithBackoff(
			grpc_retry.BackoffExponentialWithJitter(time.Second, 2.0)),
	)
}
