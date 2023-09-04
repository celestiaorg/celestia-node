package pruner

import (
	"context"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	libhead "github.com/celestiaorg/go-header"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("pruner")

type Config struct {
	RecencyWindow uint64
	BatchSize    uint64
}

type StoragePruner struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg *Config

	edsStore *eds.Store
	hsub     libhead.Subscriber[*header.ExtendedHeader]
	da 		 share.Availability
	ds       datastore.Batching

	networkHead      uint64
	lastPrunedHeight uint64
}

func NewStoragePruner(edsStore *eds.Store, hsub libhead.Subscriber[*header.ExtendedHeader], ds datastore.Batching) *StoragePruner {
	return &StoragePruner{
		edsStore: edsStore,
		hsub:     hsub,
		ds:       ds,
	}
}

func (sp *StoragePruner) Start(ctx context.Context) error {
	sp.ctx, sp.cancel = context.WithCancel(context.Background())

	sub, err := sp.hsub.Subscribe()
	if err != nil {
		return err
	}

	go sp.listenForHeaders(sp.ctx, sub)
	return nil
}

func (sp *StoragePruner) Stop(ctx context.Context) error {
	// TODO: Should we do done channels and a select to ensure services are fully stopped before returning?
	sp.cancel()
	return nil
}

func (sp *StoragePruner) SampleAndRegister(ctx context.Context, h *header.ExtendedHeader) error {
	
}

func (sp *StoragePruner) listenForHeaders(ctx context.Context, sub libhead.Subscription[*header.ExtendedHeader]) {
	for {
		h, err := sub.NextHeader(ctx)
		if err != nil {
			if err == context.Canceled {
				return
			}

			log.Errorw("failed to get next header", "err", err)
			continue
		}
		newHeight := h.Height()
		if newHeight <= sp.networkHead {
			log.Warnf("received head height: %v, which is lower or the same as previously known: %v", newHeight, sp.networkHead)
			continue
		}

		sp.networkHead = newHeight
	}
}
