package das

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/service/share"
)

var log = logging.Logger("das")

// DASer continuously validates availability of data committed to headers.
type DASer struct {
	da    share.Availability
	bcast fraud.Broadcaster
	hsub  header.Subscriber

	// getter allows the DASer to fetch an ExtendedHeader (EH) at a certain height
	// and blocks until the EH has been processed by the header store.
	getter header.Getter
	// checkpoint store
	cstore datastore.Datastore

	ctx    context.Context
	cancel context.CancelFunc

	jobsCh chan *catchUpJob

	sampleDn  chan struct{} // done signal for sample loop
	catchUpDn chan struct{} // done signal for catchUp loop
}

// NewDASer creates a new DASer.
func NewDASer(
	da share.Availability,
	hsub header.Subscriber,
	getter header.Getter,
	cstore datastore.Datastore,
	bcast fraud.Broadcaster,
) *DASer {
	wrappedDS := wrapCheckpointStore(cstore)
	return &DASer{
		da:        da,
		bcast:     bcast,
		hsub:      hsub,
		getter:    getter,
		cstore:    wrappedDS,
		jobsCh:    make(chan *catchUpJob, 16),
		sampleDn:  make(chan struct{}),
		catchUpDn: make(chan struct{}),
	}
}

// Start initiates subscription for new ExtendedHeaders and spawns a sampling routine.
func (d *DASer) Start(context.Context) error {
	if d.cancel != nil {
		return fmt.Errorf("da: DASer already started")
	}

	sub, err := d.hsub.Subscribe()
	if err != nil {
		return err
	}

	d.ctx, d.cancel = context.WithCancel(context.Background())

	// load latest DASed checkpoint
	checkpoint, err := loadCheckpoint(d.ctx, d.cstore)
	if err != nil {
		return err
	}
	log.Infow("loaded checkpoint", "height", checkpoint)

	// kick off catch-up routine manager
	go d.catchUpManager(d.ctx, checkpoint)
	// kick off sampling routine for recently received headers
	go d.sample(d.ctx, sub, checkpoint)
	return nil
}

// Stop stops sampling.
func (d *DASer) Stop(ctx context.Context) error {
	// Stop func can now be invoked twice in one Lifecycle, when:
	// * BEFP is received;
	// * node is stopping;
	// this statement helps avoiding panic on the second Stop.
	// If ctx.Err is not nil then it means that Stop was called for the first time.
	// NOTE: we are expecting *only* ContextCancelled error here.
	if d.ctx.Err() == context.Canceled {
		return nil
	}
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
	defer func() {
		sub.Cancel()
		// send done signal
		d.sampleDn <- struct{}{}
	}()

	// sampleHeight tracks the last successful height of this routine
	sampleHeight := checkpoint

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
		if h.Height > sampleHeight+1 {
			// DAS headers between last DASed height up to the current
			// header
			job := &catchUpJob{
				from: sampleHeight,
				to:   h.Height - 1,
			}
			select {
			case <-ctx.Done():
				return
			case d.jobsCh <- job:
			}
		}

		err = d.sampleHeader(ctx, h)
		if err != nil {
			return
		}

		sampleHeight = h.Height
	}
}

// catchUpJob represents a catch-up job. (from:to]
type catchUpJob struct {
	from, to int64
}

// catchUpManager manages catch-up jobs, performing them one at a time, exiting
// only once context is canceled and storing latest DASed checkpoint to disk.
func (d *DASer) catchUpManager(ctx context.Context, checkpoint int64) {
	defer func() {
		// store latest DASed checkpoint to disk here to ensure that if DASer is not yet
		// fully caught up to network head, it will resume DASing from this checkpoint
		// up to current network head
		// TODO @renaynay: Implement Share Cache #180 to ensure no duplicate DASing over same
		//  header
		if err := storeCheckpoint(ctx, d.cstore, checkpoint); err != nil {
			log.Errorw("storing checkpoint to disk", "height", checkpoint, "err", err)
		}
		log.Infow("stored checkpoint to disk", "checkpoint", checkpoint)
		// signal that catch-up routine finished
		d.catchUpDn <- struct{}{}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case job := <-d.jobsCh:
			// perform catchUp routine
			height, err := d.catchUp(ctx, job)
			// store the height of the last successfully sampled header
			checkpoint = height
			// exit routine if a catch-up job was unsuccessful
			if err != nil {
				log.Errorw("catch-up routine failed", "attempted range: from", job.from, "to", job.to)
				return
			}
		}
	}
}

// catchUp starts a sampling routine for headers starting at the next header
// after the `from` height and exits the loop once `to` is reached. (from:to]
func (d *DASer) catchUp(ctx context.Context, job *catchUpJob) (int64, error) {
	routineStartTime := time.Now()
	log.Infow("sampling past headers", "from", job.from, "to", job.to)

	// start sampling from height at checkpoint+1 since the
	// checkpoint height is DASed by broader sample routine
	for height := job.from + 1; height <= job.to; height++ {
		h, err := d.getter.GetByHeight(ctx, uint64(height))
		if err != nil {
			if err == context.Canceled {
				// report previous height as the last successfully sampled height and
				// error as nil since the routine was ordered to stop
				return height - 1, nil
			}

			log.Errorw("failed to get next header", "height", height, "err", err)
			// report previous height as the last successfully sampled height
			return height - 1, err
		}

		err = d.sampleHeader(ctx, h)
		if err != nil {
			return h.Height - 1, err
		}
	}

	log.Infow("successfully sampled past headers", "from", job.from,
		"to", job.to, "finished (s)", time.Since(routineStartTime))
	// report successful result
	return job.to, nil
}

func (d *DASer) sampleHeader(ctx context.Context, h *header.ExtendedHeader) error {
	startTime := time.Now()

	err := d.da.SharesAvailable(ctx, h.DAH)
	if err != nil {
		if err == context.Canceled {
			return nil
		}
		var byzantineErr *ipld.ErrByzantine
		if errors.As(err, &byzantineErr) {
			log.Warn("Propagating proof...")
			sendErr := d.bcast.Broadcast(ctx, fraud.CreateBadEncodingProof(h.Hash(), uint64(h.Height), byzantineErr))
			if sendErr != nil {
				log.Errorw("fraud proof propagating failed", "err", sendErr)
			}
		}
		log.Errorw("sampling failed", "height", h.Height, "hash", h.Hash(),
			"square width", len(h.DAH.RowsRoots), "data root", h.DAH.Hash(), "err", err)
		log.Warn("DASer WILL BE STOPPED. IN ORDER TO CONTINUE SAMPLING, RE-START THE NODE")
		// report previous height as the last successfully sampled height
		return err
	}

	sampleTime := time.Since(startTime)
	log.Infow("sampled header", "height", h.Height, "hash", h.Hash(),
		"square width", len(h.DAH.RowsRoots), "finished (s)", sampleTime.Seconds())

	return nil
}
