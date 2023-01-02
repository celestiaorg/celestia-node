package fraud

import (
	"github.com/ipfs/go-datastore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func newFraudService(syncerEnabled bool) func(
	fx.Lifecycle,
	*pubsub.PubSub,
	host.Host,
	libhead.Store[*header.ExtendedHeader],
	datastore.Batching,
	p2p.Network,
) (Module, fraud.Service, error) {
	return func(
		lc fx.Lifecycle,
		sub *pubsub.PubSub,
		host host.Host,
		hstore libhead.Store[*header.ExtendedHeader],
		ds datastore.Batching,
		network p2p.Network,
	) (Module, fraud.Service, error) {
		pservice := fraud.NewProofService(sub, host, hstore.GetByHeight, ds, syncerEnabled, string(network))
		lc.Append(fx.Hook{
			OnStart: pservice.Start,
			OnStop:  pservice.Stop,
		})
		return &Service{
			Service: pservice,
		}, pservice, nil
	}
}
