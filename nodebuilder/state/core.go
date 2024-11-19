package state

import (
	"errors"
	"os"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	libfraud "github.com/celestiaorg/go-fraud"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/state"
)

// coreAccessor constructs a new instance of state.Module over
// a celestia-core connection.
func coreAccessor(
	corecfg core.Config,
	keyring keyring.Keyring,
	keyname AccountName,
	sync *sync.Syncer[*header.ExtendedHeader],
	fraudServ libfraud.Service[*header.ExtendedHeader],
	network p2p.Network,
	opts []state.Option,
) (
	*state.CoreAccessor,
	Module,
	*modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader],
	error,
) {
	if corecfg.TLSEnabled {
		tlsCfg, err := core.TLS(corecfg.TLSPath)
		switch {
		case err == nil:
		case errors.Is(err, os.ErrNotExist):
			// set an empty config if path is empty under `TLSEnabled=true`
			tlsCfg = core.EmptyTLSConfig()
		default:
			return nil, nil, nil, err
		}

		xtoken, err := core.XToken(corecfg.XTokenPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil, err
		}
		opts = append(opts, state.WithTLSConfig(tlsCfg), state.WithXToken(xtoken))
	}

	ca, err := state.NewCoreAccessor(keyring, string(keyname), sync,
		corecfg.IP, corecfg.GRPCPort, network.String(), opts...)

	sBreaker := &modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]{
		Service:   ca,
		FraudType: byzantine.BadEncoding,
		FraudServ: fraudServ,
	}
	return ca, ca, sBreaker, err
}
