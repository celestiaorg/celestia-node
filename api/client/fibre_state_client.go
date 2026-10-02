package client

import (
	"context"
	"fmt"
	"math"
	"strings"

	coregrpc "github.com/cometbft/cometbft/rpc/grpc"
	core "github.com/cometbft/cometbft/types"
	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"

	appstate "github.com/celestiaorg/celestia-app/v10/fibre/state"
	"github.com/celestiaorg/celestia-app/v10/fibre/validator"
	fibretypes "github.com/celestiaorg/celestia-app/v10/x/fibre/types"
	valaddr "github.com/celestiaorg/celestia-app/v10/x/valaddr/types"
)

// grpcStateClient is an [appstate.Client] that runs all queries over an existing
// connection, so it uses that connection's TLS and auth settings. Like the app's
// default state client, it trusts the responses of the core endpoint.
type grpcStateClient struct {
	blockCli   coregrpc.BlockAPIClient
	valaddrCli valaddr.QueryClient
	fibreCli   fibretypes.QueryClient
	nodeCli    tmservice.ServiceClient

	chainID string
}

var _ appstate.Client = (*grpcStateClient)(nil)

// newGRPCStateClient builds a [grpcStateClient] over conn.
func newGRPCStateClient(conn *grpc.ClientConn) *grpcStateClient {
	return &grpcStateClient{
		blockCli:   coregrpc.NewBlockAPIClient(conn),
		valaddrCli: valaddr.NewQueryClient(conn),
		fibreCli:   fibretypes.NewQueryClient(conn),
		nodeCli:    tmservice.NewServiceClient(conn),
	}
}

// Head returns the latest validator set.
func (c *grpcStateClient) Head(ctx context.Context) (validator.Set, error) {
	return c.getByHeight(ctx, 0)
}

// GetByHeight returns the validator set at the given height. Height must be
// greater than 0; use Head for the latest set.
func (c *grpcStateClient) GetByHeight(ctx context.Context, height uint64) (validator.Set, error) {
	if height == 0 {
		return validator.Set{}, fmt.Errorf("height must be greater than 0, use Head() to get the latest validator set")
	}
	return c.getByHeight(ctx, height)
}

func (c *grpcStateClient) getByHeight(ctx context.Context, height uint64) (validator.Set, error) {
	resp, err := c.blockCli.ValidatorSet(ctx, &coregrpc.ValidatorSetRequest{Height: int64(height)})
	if err != nil {
		return validator.Set{}, fmt.Errorf("getting validator set at height %d: %w", height, err)
	}
	if resp.ValidatorSet == nil {
		return validator.Set{}, fmt.Errorf("validator set is nil in response for height %d", height)
	}
	set, err := core.ValidatorSetFromProto(resp.ValidatorSet)
	if err != nil {
		return validator.Set{}, fmt.Errorf("converting validator set from proto at height %d: %w", height, err)
	}
	return validator.Set{ValidatorSet: set, Height: uint64(resp.Height)}, nil
}

// GetHost resolves a validator's fibre provider host from on-chain state.
func (c *grpcStateClient) GetHost(ctx context.Context, val *core.Validator) (validator.Host, error) {
	consAddr := sdk.ConsAddress(val.Address.Bytes()).String()
	resp, err := c.valaddrCli.FibreProviderInfo(ctx, &valaddr.QueryFibreProviderInfoRequest{
		ValidatorConsensusAddress: consAddr,
	})
	if err != nil {
		return "", fmt.Errorf("querying fibre provider info for validator %s: %w", consAddr, err)
	}
	if !resp.GetFound() {
		return "", fmt.Errorf("host not found for validator %s", consAddr)
	}
	host := validator.Host(resp.GetInfo().GetHost())
	if err := valaddr.ValidateHost(host.String()); err != nil {
		return "", fmt.Errorf("got invalid host %s: %w", host.String(), err)
	}
	return host, nil
}

// ChainID returns the chain ID resolved by Start.
func (c *grpcStateClient) ChainID() string { return c.chainID }

// VerifyPromise validates a payment promise against on-chain state.
func (c *grpcStateClient) VerifyPromise(
	ctx context.Context, promise *appstate.PaymentPromise,
) (appstate.VerifiedPromise, error) {
	resp, err := c.fibreCli.ValidatePaymentPromise(ctx, &fibretypes.QueryValidatePaymentPromiseRequest{Promise: *promise})
	if err != nil {
		return appstate.VerifiedPromise{}, err
	}
	if !resp.IsValid {
		return appstate.VerifiedPromise{}, fmt.Errorf("payment promise is invalid")
	}
	if resp.ExpirationTime == nil {
		return appstate.VerifiedPromise{}, fmt.Errorf("expiration time not provided in validation response")
	}
	return appstate.VerifiedPromise{
		ExpiresAt:      *resp.ExpirationTime,
		ShardRetention: resp.ShardRetention,
	}, nil
}

// FullStakeStorageBudget returns the FullStakeStorageBudget governance parameter.
func (c *grpcStateClient) FullStakeStorageBudget(ctx context.Context) (int64, error) {
	resp, err := c.fibreCli.Params(ctx, &fibretypes.QueryParamsRequest{})
	if err != nil {
		return 0, err
	}
	budget := resp.Params.FullStakeStorageBudget
	if budget > math.MaxInt64 {
		return math.MaxInt64, nil
	}
	return int64(budget), nil
}

// Start resolves the chain ID from the connection's node info.
func (c *grpcStateClient) Start(ctx context.Context) error {
	resp, err := c.nodeCli.GetNodeInfo(ctx, &tmservice.GetNodeInfoRequest{})
	if err != nil {
		return fmt.Errorf("detect chain ID: %w", err)
	}
	if resp.GetDefaultNodeInfo() == nil {
		return fmt.Errorf("detect chain ID: missing node info in gRPC response")
	}
	chainID := strings.TrimSpace(resp.GetDefaultNodeInfo().GetNetwork())
	if chainID == "" {
		return fmt.Errorf("detect chain ID: empty chain ID in node info response")
	}
	c.chainID = chainID
	return nil
}

// Stop is a no-op: the gRPC conn is owned and closed elsewhere (Client.closer).
func (c *grpcStateClient) Stop(context.Context) error { return nil }
