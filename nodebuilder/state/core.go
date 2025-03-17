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
// celestia-core connections.
func coreAccessor(
	keyring keyring.Keyring,
	keyname AccountName,
	sync *sync.Syncer[*header.ExtendedHeader],
	fraudServ libfraud.Service[*header.ExtendedHeader],
	network p2p.Network,
	clients []*grpc.ClientConn,
) (
	*state.CoreAccessor,
	Module,
	*modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader],
	error,
) {
	ca, err := state.NewCoreAccessor(keyring, string(keyname), sync, clients, network.String())

	sBreaker := &modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]{
		Service:   ca,
		FraudType: byzantine.BadEncoding,
		FraudServ: fraudServ,
	}
	return ca, ca, sBreaker, err
}
