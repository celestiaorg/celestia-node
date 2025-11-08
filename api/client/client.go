package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	logging "github.com/ipfs/go-log/v2"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	stateapi "github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/state"
)

var log = logging.Logger("celestia-client")

// Config holds configuration for the Client.
type Config struct {
	ReadConfig   ReadConfig
	SubmitConfig SubmitConfig
}

type SubmitConfig struct {
	DefaultKeyName string
	Network        p2p.Network
	CoreGRPCConfig CoreGRPCConfig
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

func (cfg Config) Validate() error {
	if err := cfg.ReadConfig.Validate(); err != nil {
		return err
	}
	return cfg.SubmitConfig.Validate()
}

func (cfg SubmitConfig) Validate() error {
	if cfg.DefaultKeyName == "" {
		return errors.New("default key name should not be empty")
	}

	if cfg.TxWorkerAccounts < 0 {
		return fmt.Errorf("worker accounts must be non-negative")
	}

	return cfg.CoreGRPCConfig.Validate()
}

// Client is a simplified Celestia client to submit blobs and interact with DA RPC.
type Client struct {
	ReadClient
	State stateapi.Module

	closer func() error
}

// MultiClient is a Celestia client with support for multiple endpoints
type MultiClient struct {
	*Client
	multiReadClient *MultiReadClient
	multiGRPCClient *MultiGRPCClient
}

// New initializes the Celestia client. It connects to the Celestia consensus nodes and Bridge
// nodes. Any changes to the keyring are not visible to the client. The client needs to be
// reinitialized to pick up new keys. Client should be closed after use by calling Close().
func New(ctx context.Context, cfg Config, kr keyring.Keyring) (*Client, error) {
	rc, err := NewReadClient(ctx, cfg.ReadConfig)
	if err != nil {
		return nil, err
	}

	cl := &Client{
		ReadClient: *rc,
	}

	err = cfg.Validate()
	if err != nil {
		return nil, err
	}
	if kr == nil {
		return nil, errors.New("keyring is nil")
	}

	grpcCl, err := grpcClient(cfg.SubmitConfig.CoreGRPCConfig)
	if err != nil {
		return nil, err
	}
	err = cl.initTxClient(ctx, cfg.SubmitConfig, grpcCl, kr)
	if err != nil {
		clerr := cl.ReadClient.Close()
		return nil, errors.Join(err, clerr)
	}
	return cl, nil
}

// NewMultiEndpoint initializes a Celestia client with support for multiple endpoints.
// It provides failover capabilities for both bridge DA and core gRPC connections.
func NewMultiEndpoint(ctx context.Context, cfg Config, kr keyring.Keyring) (*MultiClient, error) {
	// Create multi-read client for bridge DA endpoints
	multiReadClient, err := NewMultiReadClient(ctx, cfg.ReadConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create multi-read client: %w", err)
	}

	// Get the primary read client
	primaryReadClient := multiReadClient.GetClient()
	if primaryReadClient == nil {
		return nil, errors.New("no read clients available")
	}

	// Create the base client with primary read client
	baseClient := &Client{
		ReadClient: *primaryReadClient,
	}

	// Validate config
	err = cfg.Validate()
	if err != nil {
		multiReadClient.Close()
		return nil, err
	}
	if kr == nil {
		multiReadClient.Close()
		return nil, errors.New("keyring is nil")
	}

	// Create multi-gRPC client for core endpoints
	multiGRPCClient, err := NewMultiGRPCClient(cfg.SubmitConfig.CoreGRPCConfig)
	if err != nil {
		multiReadClient.Close()
		return nil, fmt.Errorf("failed to create multi-gRPC client: %w", err)
	}

	// Initialize transaction client with primary gRPC connection
	err = baseClient.initTxClient(ctx, cfg.SubmitConfig, multiGRPCClient.GetConnection(), kr)
	if err != nil {
		multiReadClient.Close()
		multiGRPCClient.Close()
		return nil, fmt.Errorf("failed to initialize transaction client: %w", err)
	}

	// Create multi-client
	multiClient := &MultiClient{
		Client:          baseClient,
		multiReadClient: multiReadClient,
		multiGRPCClient: multiGRPCClient,
	}

	// Override the closer to close all connections
	multiClient.closer = func() error {
		var errs []error

		// Close multi-read client
		if err := multiReadClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close multi-read client: %w", err))
		}

		// Close multi-gRPC client
		if err := multiGRPCClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close multi-gRPC client: %w", err))
		}

		// Note: State service is managed by the ServiceBreaker in nodebuilder
		// and doesn't need explicit stopping in the client

		if len(errs) > 0 {
			return errors.Join(errs...)
		}
		return nil
	}

	return multiClient, nil
}

// GetReadClients returns all available read clients for advanced usage
func (mc *MultiClient) GetReadClients() []*ReadClient {
	return mc.multiReadClient.GetAllClients()
}

// GetGRPCConnections returns all available gRPC connections for advanced usage
func (mc *MultiClient) GetGRPCConnections() []*grpc.ClientConn {
	return mc.multiGRPCClient.GetAllConnections()
}

func (c *Client) initTxClient(
	ctx context.Context,
	submitCfg SubmitConfig,
	conn *grpc.ClientConn,
	kr keyring.Keyring,
) error {
	var opts []state.Option
	if submitCfg.TxWorkerAccounts > 0 {
		opts = append(opts, state.WithTxWorkerAccounts(submitCfg.TxWorkerAccounts))
	}

	// key is specified. Set up core accessor and txClient
	core, err := state.NewCoreAccessor(
		kr,
		submitCfg.DefaultKeyName,
		trustedHeadGetter{remote: c.Header},
		conn,
		submitCfg.Network.String(),
		nil,
		opts...,
	)
	if err != nil {
		return err
	}
	err = core.Start(ctx)
	if err != nil {
		return err
	}
	c.State = core

	// setup blob submission service using core
	blobSvc := blob.NewService(core, nil, nil, nil)
	err = blobSvc.Start(ctx)
	if err != nil {
		return err
	}
	c.Blob = &blobSubmitClient{
		Module:    c.Blob,
		submitter: blobSvc,
	}

	c.closer = func() error {
		err = conn.Close()
		if err != nil {
			return fmt.Errorf("failed to close grpc connection: %w", err)
		}
		err = core.Stop(ctx)
		if err != nil {
			return fmt.Errorf("failed to stop core accessor: %w", err)
		}
		err = blobSvc.Stop(ctx)
		if err != nil {
			return fmt.Errorf("failed to stop blob service: %w", err)
		}
		return nil
	}
	return nil
}

// Close closes all open connections to Celestia consensus nodes and Bridge nodes.
func (c *Client) Close() error {
	err := c.ReadClient.Close()
	if err != nil {
		return fmt.Errorf("failed to close read client: %w", err)
	}
	return c.closer()
}
