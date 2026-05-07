package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"

	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/state"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

func newTxClient(
	cfg Config,
	keyring keyring.Keyring,
	keyname AccountName,
	client *grpc.ClientConn,
	additionalConns core.AdditionalCoreConns,
) (*txclient.TxClient, error) {
	var opts []txclient.Option
	if len(additionalConns) > 0 {
		opts = append(opts, txclient.WithAdditionalCoreEndpoints(additionalConns))
	}

	if cfg.EstimatorAddress != "" {
		opts = append(opts, txclient.WithEstimatorService(cfg.EstimatorAddress))

		if cfg.EnableEstimatorTLS {
			opts = append(opts, txclient.WithEstimatorServiceTLS())
		}
	}

	if cfg.TxWorkerAccounts > 0 {
		opts = append(opts, txclient.WithTxWorkerAccounts(cfg.TxWorkerAccounts))
	}

	return txclient.NewTxClient(keyring, string(keyname), client, opts...)
}

// coreAccessor constructs a new instance of state.Module over
// a celestia-core connection.
func coreAccessor(
	tc *txclient.TxClient,
	keyring keyring.Keyring,
	keyname AccountName,
	sync *sync.Syncer[*header.ExtendedHeader],
	network p2p.Network,
	client *grpc.ClientConn,
) (
	*state.CoreAccessor,
	Module,
	error,
) {
	ca, err := state.NewCoreAccessor(tc, keyring, string(keyname), sync, client, network.String())
	return ca, ca, err
}
