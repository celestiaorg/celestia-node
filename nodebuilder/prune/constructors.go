package prune

import (
	"github.com/ipfs/go-datastore"

	hdr "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/pruner"
)

func newPrunerService(
	p pruner.Pruner,
	window pruner.AvailabilityWindow,
	getter hdr.Store[*header.ExtendedHeader],
	ds datastore.Batching,
	opts ...pruner.Option,
) *pruner.Service {
	return pruner.NewService(p, window, getter, ds, p2p.BlockTime, opts...)
}
