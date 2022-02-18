package das

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

var log = logging.Logger("das")

// DASer continuously validates availability of data committed to headers.
type DASer struct {
	da   share.Availability
	hsub header.Subscriber

	// getter allows the DASer to fetch a header at a certain height
	// and blocks until it becomes available.
	getter HeaderGetter
	// checkpoint from disk -- DASStore stores checkpoint w/ DASCheckpoint key
	// checkpoint = latest successfully DASed header (reference to it, not the header)
	ds datastore.Datastore

	cancel             context.CancelFunc
	sampleLatestDn     chan struct{} // done signal for sampleLatest loop
	sampleCheckpointDn chan struct{} // done signal for sampleFromCheckpoint loop
}

// NewDASer creates a new DASer.
func NewDASer(
	da share.Availability,
	hsub header.Subscriber,
	getter HeaderGetter,
	ds datastore.Datastore,
) *DASer {
	return &DASer{
		da:                 da,
		hsub:               hsub,
		getter:             getter,
		ds:                 ds,
		sampleLatestDn:     make(chan struct{}),
		sampleCheckpointDn: make(chan struct{}),
	}
}

// Start initiates subscription for new ExtendedHeaders and spawns a sampling routine.
func (d *DASer) Start(ctx context.Context) error {
	if d.cancel != nil {
		return fmt.Errorf("da: DASer already started")
	}

	sub, err := d.hsub.Subscribe()
	if err != nil {
		return err
	}

	// load latest DASed checkpoint
	checkpoint, err := loadCheckpoint(d.ds)
	if err != nil {
		return err
	}
	log.Infow("loaded latest DASed checkpoint", "height", checkpoint)

	// load current network head
	// TODO @renaynay: do we have to sample over netHead as well or will that be handled by sampleLatest via hsub?
	netHead, err := d.getter.Head(ctx)
	if err != nil {
		return err
	}

	dasCtx, cancel := context.WithCancel(context.Background())

	// start two separate routines:
	// 1. samples headers from the latest DASed header to the
	//    current network head (samples headers from the past)
	// 2. samples new headers coming through the ExtendedHeader
	//    gossipsub topic (samples new inbound headers in the
	//    network)
	go d.sampleFromCheckpoint(dasCtx, checkpoint, netHead.Height)
	go d.sampleLatest(dasCtx, sub, checkpoint)

	d.cancel = cancel
	return nil
}

// Stop stops sampling.
func (d *DASer) Stop(ctx context.Context) error {
	d.cancel()
	// wait for both sampling routines to exit
	for i := 0; i < 2; i++ {
		select {
		// TODO @renaynay: check to ensure if sampleCheckpoint already done when stop called, then still works
		case <-d.sampleCheckpointDn:
			continue
		case <-d.sampleLatestDn:
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	d.cancel = nil
	return nil
}

// sampleFromCheckpoint starts sampling headers from the last known
// `checkpoint` (latest/heighest DASed header), and breaks the loop
// once network head `netHead` is reached, leaving only the `sampleLatest`
// loop running.
//
// 2. Loop:
// 2a. request by height (checkpoint+1)
// 2b. DAS on that height
// 2c. stop when height == network height
func (d *DASer) sampleFromCheckpoint(ctx context.Context, checkpoint, netHead int64) {
	defer close(d.sampleCheckpointDn)
	// immediately break if DASer is up to speed with network head
	if checkpoint+1 == netHead { // TODO @renaynay: test this scenario
		log.Debugw("caught up to network head", "net head", netHead)
		return
	}

	// start sampling from the next header after the checkpoint as the
	// checkpoint has already been successfully DASed.
	for height := checkpoint + 1; height < netHead; height++ {
		h, err := d.getter.GetByHeight(ctx, uint64(height))
		if err != nil {
			if err == context.Canceled {
				return
			}

			log.Errorw("failed to get next header", "height", height, "err", err)
			continue // TODO @renaynay: should we really continue in this case?
		}

		startTime := time.Now()

		err = d.da.SharesAvailable(ctx, h.DAH)
		if err != nil {
			if err == context.Canceled {
				return
			}
			log.Errorw("sampling failed", "height", h.Height, "hash", h.Hash(),
				"square width", len(h.DAH.RowsRoots), "data root", h.DAH.Hash(), "err", err)
			// continue sampling
		}

		sampleTime := time.Since(startTime)
		log.Infow("sampling successful", "height", h.Height, "hash", h.Hash(),
			"square width", len(h.DAH.RowsRoots), "finished (s)", sampleTime.Seconds())
	}
}

// sampleLatest validates availability for each Header received from header subscription.
func (d *DASer) sampleLatest(ctx context.Context, sub header.Subscription, checkpoint int64) {
	height := checkpoint

	defer func() {
		// store latest DASed checkpoint to disk
		// TODO @renaynay: what sampleLatest DASes [100:150] and
		//  stores latest checkpoint to disk as network head (150)
		// 	but sampleFromCheckpoint routine has only sampled from [1:40] so there is a gap
		//  missing from (40: 100)?
		if err := storeCheckpoint(d.ds, height); err != nil {
			log.Errorw("storing latest DASed checkpoint to disk", "height", height, "err", err)
		}
		sub.Cancel()
		close(d.sampleLatestDn)
	}()

	for {
		h, err := sub.NextHeader(ctx)
		if err != nil {
			if err == context.Canceled {
				return
			}

			log.Errorw("failed to get next header", "err", err)
			continue
		}

		startTime := time.Now()

		err = d.da.SharesAvailable(ctx, h.DAH)
		if err != nil {
			if err == context.Canceled {
				return
			}
			log.Errorw("sampling failed", "height", h.Height, "hash", h.Hash(),
				"square width", len(h.DAH.RowsRoots), "data root", h.DAH.Hash(), "err", err)
			// continue sampling
		}

		sampleTime := time.Since(startTime)
		log.Infow("sampling successful", "height", h.Height, "hash", h.Hash(),
			"square width", len(h.DAH.RowsRoots), "finished (s)", sampleTime.Seconds())

		height = h.Height
	}
}
