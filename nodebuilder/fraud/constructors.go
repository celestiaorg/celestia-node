package fraud

import (
	"github.com/ipfs/go-datastore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/go-fraud"
	"github.com/celestiaorg/go-fraud/fraudserv"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func fraudUnmarshaler() fraud.ProofUnmarshaler[*header.ExtendedHeader] {
	return defaultProofUnmarshaler
}

func newFraudServiceWithSync(
	lc fx.Lifecycle,
	sub *pubsub.PubSub,
	host host.Host,
	hstore libhead.Store[*header.ExtendedHeader],
	registry fraud.ProofUnmarshaler[*header.ExtendedHeader],
	ds datastore.Batching,
	network p2p.Network,
) (Module, fraud.Service[*header.ExtendedHeader], error) {
	syncerEnabled := true
	pservice := fraudserv.NewProofService(sub, host, hstore.GetByHeight, registry, ds, syncerEnabled, network.String())
	lc.Append(fx.Hook{
		OnStart: pservice.Start,
		OnStop:  pservice.Stop,
	})
	return &module{
		Service: pservice,
	}, pservice, nil
}

func newFraudServiceWithoutSync(
	lc fx.Lifecycle,
	sub *pubsub.PubSub,
	host host.Host,
	hstore libhead.Store[*header.ExtendedHeader],
	registry fraud.ProofUnmarshaler[*header.ExtendedHeader],
	ds datastore.Batching,
	network p2p.Network,
) (Module, fraud.Service[*header.ExtendedHeader], error) {
	syncerEnabled := false
	pservice := fraudserv.NewProofService(sub, host, hstore.GetByHeight, registry, ds, syncerEnabled, network.String())
	lc.Append(fx.Hook{
		OnStart: pservice.Start,
		OnStop:  pservice.Stop,
	})
	return &module{
		Service: pservice,
	}, pservice, nil
}
