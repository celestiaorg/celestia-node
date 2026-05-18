package pruner

import (
	"github.com/ipfs/go-datastore"

	libhead "github.com/celestiaorg/go-header"
	headsync "github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	modshare "github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/pruner"
)

// ensures Pruner always starts after Syncer
type syncerAnchor = headsync.Syncer[*header.ExtendedHeader]

func newPrunerService(
	p pruner.Pruner,
	window modshare.Window,
	getter libhead.Store[*header.ExtendedHeader],
	_ *syncerAnchor,
	ds datastore.Batching,
	opts ...pruner.Option,
) (*pruner.Service, error) {
	serv, err := pruner.NewService(p, window.Duration(), getter, ds, p2p.BlockTime, opts...)
	if err != nil {
		return nil, err
	}

	if MetricsEnabled {
		err := pruner.WithPrunerMetrics(serv)
		if err != nil {
			return nil, err
		}
	}

	return serv, nil
}
