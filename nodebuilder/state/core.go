package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"

	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/state"
)

// coreAccessor constructs a new instance of state.Module over
// a celestia-core connection.
func coreAccessor(
	cfg Config,
	keyring keyring.Keyring,
	keyname AccountName,
	sync *sync.Syncer[*header.ExtendedHeader],
	network p2p.Network,
	client *grpc.ClientConn,
	additionalConns core.AdditionalCoreConns,
) (
	*state.CoreAccessor,
	Module,
	error,
) {
	var opts []state.Option
	if len(additionalConns) > 0 {
		opts = append(opts, state.WithAdditionalCoreEndpoints(additionalConns))
	}

	if cfg.EstimatorAddress != "" {
		opts = append(opts, state.WithEstimatorService(cfg.EstimatorAddress))

		if cfg.EnableEstimatorTLS {
			opts = append(opts, state.WithEstimatorServiceTLS())
		}
	}

	if cfg.TxWorkerAccounts > 0 {
		opts = append(opts, state.WithTxWorkerAccounts(cfg.TxWorkerAccounts))
	}

	ca, err := state.NewCoreAccessor(keyring, string(keyname), sync, client, network.String(), nil, opts...)

	return ca, ca, err
}
