package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"

	libfraud "github.com/celestiaorg/go-fraud"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/state"
)

// coreAccessor constructs a new instance of state.Module over
// a celestia-core connection.
func coreAccessor(
	cfg Config,
	keyring keyring.Keyring,
	keyname AccountName,
	sync *sync.Syncer[*header.ExtendedHeader],
	fraudServ libfraud.Service[*header.ExtendedHeader],
	network p2p.Network,
	client *grpc.ClientConn,
) (
	*state.CoreAccessor,
	Module,
	*modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader],
	error,
) {
	var opts []state.Option
	if cfg.EstimatorAddress != "" {
		opts = append(opts, state.WithEstimatorService(cfg.EstimatorAddress))

		if cfg.EnableEstimatorTLS {
			opts = append(opts, state.WithEstimatorServiceTLS())
		}
	}

	ca, err := state.NewCoreAccessor(keyring, string(keyname), sync, client, network.String(), opts...)

	sBreaker := &modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]{
		Service:   ca,
		FraudType: byzantine.BadEncoding,
		FraudServ: fraudServ,
	}
	return ca, ca, sBreaker, err
}
