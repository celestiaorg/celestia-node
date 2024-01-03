package prune

import (
	hdr "github.com/celestiaorg/go-header"
	"github.com/ipfs/go-datastore"
	"time"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/pruner"
)

func newPruner(
	p pruner.Pruner,
	window pruner.AvailabilityWindow,
	getter hdr.Getter[*header.ExtendedHeader],
	ds datastore.Datastore,
	blockTime time.Duration,
	opts ...pruner.Option,
) *pruner.Service {
	// TODO @renaynay: remove this once pruning implementation
	opts = append(opts, pruner.WithDisabledGC())
	return pruner.NewService(p, window, getter, ds, blockTime, opts...)
}
