package share

import (
	"context"
	"errors"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/cache"
	disc "github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/service"
)

func discovery(cfg Config) func(routing.ContentRouting, host.Host) *disc.Discovery {
	return func(
		r routing.ContentRouting,
		h host.Host,
	) *disc.Discovery {
		return disc.NewDiscovery(
			h,
			routingdisc.NewRoutingDiscovery(r),
			cfg.PeersLimit,
			cfg.DiscoveryInterval,
			cfg.AdvertiseInterval,
		)
	}
}

// cacheAvailability wraps either Full or Light availability with a cache for result sampling.
func cacheAvailability[A share.Availability](lc fx.Lifecycle, ds datastore.Batching, avail A) share.Availability {
	ca := cache.NewShareAvailability(avail, ds)
	lc.Append(fx.Hook{
		OnStop: ca.Close,
	})
	return ca
}

func newModule(lc fx.Lifecycle, bServ blockservice.BlockService, avail share.Availability) Module {
	serv := service.NewShareService(bServ, avail)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return serv.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return serv.Stop(ctx)
		},
	})
	return &module{serv}
}

// ensureEmptyCARExists adds the empty EDS from share.EnsureEmptySquareExists to the provided EDS store.
func ensureEmptyCARExists(ctx context.Context, store *eds.Store) error {
	bServ := blockservice.New(store.Blockstore(), nil)
	// we ignore the error because we know that the batchAdder will not be able to commit to our
	// Blockstore, which is not meant for writes.
	eds, _ := share.EnsureEmptySquareExists(ctx, bServ)

	dah := da.NewDataAvailabilityHeader(eds)

	err := store.Put(ctx, dah.Hash(), eds)
	if errors.Is(err, dagstore.ErrShardExists) {
		return nil
	}
	return err
}
