package client

import (
	"context"
	"errors"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	logging "github.com/ipfs/go-log/v2"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-node/blob"
	blobapi "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	blobstreamapi "github.com/celestiaorg/celestia-node/nodebuilder/blobstream"
	fraudapi "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	headerapi "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	shareapi "github.com/celestiaorg/celestia-node/nodebuilder/share"
	stateapi "github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/state"
)

var log = logging.Logger("celestia-client")

// Config holds configuration for the Client.
type Config struct {
	// TODO: Do we need backwards compatibility tracking? APIVersion will check version of the API and
	// fail if it doesn't match
	// APIVersion        string

	// TODO: do we need logging inside the client?
	// EnableLogging     bool
	// LogLevel          string

	ReadConfig   ReadConfig
	SubmitConfig SubmitConfig
}

type SubmitConfig struct {
	DefaultKeyName string
	Network        p2p.Network
	CoreGRPCConfig CoreGRPCConfig
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
	return cfg.CoreGRPCConfig.Validate()
}

// Client is a simplified Celestia client to submit blobs and interact with DA RPC.
type Client struct {
	Blob       blobapi.Module
	Header     headerapi.Module
	State      stateapi.Module
	Share      shareapi.Module
	Fraud      fraudapi.Module
	Blobstream blobstreamapi.Module

	chainCloser func() error
}

// New initializes the Celestia client. It connects to the Celestia consensus nodes and Bridge
// nodes. Any changes to the keyring are not visible to the client. The client needs to be
// reinitialized to pick up new keys.
func New(ctx context.Context, cfg Config, kr keyring.Keyring) (*Client, error) {
	c, err := NewReadClient(ctx, cfg.ReadConfig)
	if err != nil {
		return nil, err
	}

	err = cfg.Validate()
	if err != nil {
		return nil, err
	}
	if kr == nil {
		return nil, errors.New("keyring is nil")
	}

	cl, err := grpcClient(cfg.SubmitConfig.CoreGRPCConfig)
	if err != nil {
		return nil, err
	}
	err = c.initTxClient(ctx, cfg.SubmitConfig, cl, kr)
	if err != nil {
		clerr := cl.Close()
		return nil, errors.Join(err, clerr)
	}
	return c, nil
}

func (c *Client) initTxClient(
	ctx context.Context,
	submitCfg SubmitConfig,
	conn *grpc.ClientConn,
	kr keyring.Keyring,
) error {
	// key is specified. Set up core accessor and txClient
	core, err := state.NewCoreAccessor(
		kr,
		submitCfg.DefaultKeyName,
		trustedHeadGetter{remote: c.Header},
		conn,
		submitCfg.Network.String(),
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

	c.chainCloser = func(prev func() error) func() error {
		return func() error {
			err := conn.Close()
			if err != nil {
				return err
			}
			err = core.Stop(ctx)
			if err != nil {
				return err
			}
			err = blobSvc.Stop(ctx)
			if err != nil {
				return err
			}
			return prev()
		}
	}(c.chainCloser)
	return nil
}

// Close closes all open connections to Celestia consensus nodes and Bridge nodes.
func (c *Client) Close() error {
	return c.chainCloser()
}
