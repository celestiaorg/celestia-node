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

	cancel    context.CancelFunc
	sampleDn  chan struct{} // done signal for sample loop
	catchUpDn chan struct{} // done signal for catchUp loop
}

// NewDASer creates a new DASer.
func NewDASer(
	da share.Availability,
	hsub header.Subscriber,
	getter HeaderGetter,
	ds datastore.Datastore,
) *DASer {
	return &DASer{
		da:        da,
		hsub:      hsub,
		getter:    getter,
		ds:        ds,
		sampleDn:  make(chan struct{}),
		catchUpDn: make(chan struct{}),
	}
}

// Start initiates subscription for new ExtendedHeaders and spawns a sampling routine.
func (d *DASer) Start(_ context.Context) error {
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

	dasCtx, cancel := context.WithCancel(context.Background())

	go d.sample(dasCtx, sub, checkpoint)

	d.cancel = cancel
	return nil
}

// Stop stops sampling.
func (d *DASer) Stop(ctx context.Context) error {
	d.cancel()
	// wait for both sampling routines to exit
	for i := 0; i < 2; i++ {
		select {
		case <-d.catchUpDn:
		case <-d.sampleDn:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	d.cancel = nil
	return nil
}

// sample validates availability for each Header received from header subscription.
func (d *DASer) sample(ctx context.Context, sub header.Subscription, checkpoint int64) {
	height := checkpoint

	defer func() {
		// store latest DASed checkpoint to disk
		// TODO @renaynay: what sample DASes [100:150] and
		//  stores latest checkpoint to disk as network head (150)
		// 	but catchUp routine has only sampled from [1:40] so there is a gap
		//  missing from (40: 100)?
		if err := storeCheckpoint(d.ds, height); err != nil {
			log.Errorw("storing latest DASed checkpoint to disk", "height", height, "err", err)
		}
		sub.Cancel()
		close(d.sampleDn)
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

		// If the next header coming through gossipsub is not adjacent
		// to our last DASed header, kick off routine to DAS all headers
		// between last DASed header and h. This situation could occur
		// either on start or due to network latency/disconnection.
		if h.Height > (height + 1) { // TODO @renaynay: what if height = 7 and h.Height = 9 (so only need to catch up on 8
			// DAS headers between last DASed height up to the current
			// header
			go d.catchUp(ctx, height, h.Height-1)
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

// catchUp starts sampling headers from the last known
// checkpoint (latest/highest DASed header) `from`, and breaks the loop
// once network head `to` is reached. (from:to]
func (d *DASer) catchUp(ctx context.Context, from, to int64) {
	defer close(d.catchUpDn)

	routineStartTime := time.Now()
	log.Infow("starting sample routine", "from", from, "to", to)

	// start sampling from height at checkpoint+1 since the
	// checkpoint height has already been successfully DASed
	for height := from + 1; height <= to; height++ {
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

	totalElapsedTime := time.Since(routineStartTime)
	log.Infow("successfully sampled all headers up to network head", "from", from,
		"to", to, "finished (s)", totalElapsedTime.Seconds())
}
