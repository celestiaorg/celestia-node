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

// TODO:
// Major limitations:
// We still carry deps for cosmos-sdk and celestia-app replaces.
// - can be solved by moving client to a separate package and decoupling all rpcs.
// Client needs to be reinitialized on every key change due to txClient being stateful and
// maintaining copy of keys.	- can be reworked by wrapping keyring and updating txClient internal
// state on key change. Complicated Core accessor requires KeyName to be set and exist in the
// keyring. Which is not necessary imo. - require refactor of core_accessor/txClient
// Can't make submit only client, because submition is coupled with reads in blob service
// - need to split blob API into read and submit

// TODO: blobSubmit can't accept nil SubmitOptions, only pointer to empty struct!

// - decide whether to keep keyring inside the client. Potentially could allow to swap/ regen keys
// on the fly. - figure out if we use first key from the keyring or require user to specify it.

// Config holds configuration for the Client.
type Config struct {
	// TODO: Do we need backwards compatibility tracking? APIVersion will check version of the API and
	// fail if it doesn't match
	// APIVersion        string

	// TODO: do we need metrics inside the client? probably not
	// EnableMetrics     bool

	// TODO: expose retries config? default is 5 atm for submit blob
	// MaxRetries        int

	// TODO: do we need logging inside the client?
	// EnableLogging     bool
	// LogLevel          string

	ReadConfig   ReadConfig
	SubmitConfig SubmitConfig
}

type SubmitConfig struct {
	DefaultKeyName      string
	Network             p2p.Network
	ConsensusGRPCConfig GRPCConfig
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
	return cfg.ConsensusGRPCConfig.Validate()
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

	cl, err := grpcClient(cfg.SubmitConfig.ConsensusGRPCConfig)
	if err != nil {
		return nil, err
	}
	err = c.initTxClient(ctx, cfg.SubmitConfig, cl, kr)
	if err != nil {
		cl.Close()
		return nil, err
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
