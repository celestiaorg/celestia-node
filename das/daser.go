package das

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

var log = logging.Logger("das")

// TODO: parameters needs performance testing on real network to define optimal values
const (
	//  samplingRange is the maximum amount of headers processed in one job.
	samplingRange = 100

	// concurrencyLimit defines the maximum amount of sampling workers running in parallel.
	concurrencyLimit = 16

	// backgroundStoreInterval is the period of time for background checkpointStore to perform a checkpoint backup.
	backgroundStoreInterval = 10 * time.Minute

	// priorityQueueSize defines the size limit of the priority queue
	priorityQueueSize = concurrencyLimit * 4

	// genesisHeight is the height sampling will start from
	genesisHeight = 1
)

// DASer continuously validates availability of data committed to headers.
type DASer struct {
	da     share.Availability
	bcast  fraud.Broadcaster
	hsub   header.Subscriber // listens for new headers in the network
	getter header.Getter     // retrieves past headers

	sampler    *samplingCoordinator
	store      checkpointStore
	subscriber subscriber

	cancel         context.CancelFunc
	subscriberDone chan struct{}
	running        int32
}

type listenFn func(ctx context.Context, height uint64)
type sampleFn func(context.Context, *header.ExtendedHeader) error

// NewDASer creates a new DASer.
func NewDASer(
	da share.Availability,
	hsub header.Subscriber,
	getter header.Getter,
	dstore datastore.Datastore,
	bcast fraud.Broadcaster,
) *DASer {
	d := &DASer{
		da:             da,
		bcast:          bcast,
		hsub:           hsub,
		getter:         getter,
		store:          newCheckpointStore(dstore),
		subscriber:     newSubscriber(),
		subscriberDone: make(chan struct{}),
	}
	d.sampler = newSamplingCoordinator(concurrencyLimit, samplingRange, getter, d.sample)

	return d
}

// Start initiates subscription for new ExtendedHeaders and spawns a sampling routine.
func (d *DASer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&d.running, 0, 1) {
		return fmt.Errorf("da: DASer already started")
	}

	sub, err := d.hsub.Subscribe()
	if err != nil {
		return err
	}

	// load latest DASed checkpoint
	cp, err := d.store.load(ctx)
	if err != nil {
		log.Warnw("checkpoint not found, initializing with height 1")

		cp = checkpoint{
			SampleFrom:  genesisHeight,
			NetworkHead: genesisHeight,
		}

		// attempt to get head info. No need to handle error, later DASer
		// will be able to find new head from subscriber after it is started
		if h, err := d.getter.Head(ctx); err == nil {
			cp.NetworkHead = uint64(h.Height)
		}
	}
	log.Info("starting DASer from checkpoint: ", cp.String())

	runCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	go d.sampler.run(runCtx, cp)
	go d.subscriber.run(runCtx, sub, d.sampler.listen)
	go d.store.runBackgroundStore(runCtx, backgroundStoreInterval, d.sampler.getCheckpoint)

	return nil
}

// Stop stops sampling.
func (d *DASer) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&d.running, 1, 0) {
		return nil
	}

	// try to store checkpoint without waiting for coordinator and workers to stop
	cp, err := d.sampler.getCheckpoint(ctx)
	if err != nil {
		log.Error("DASer coordinator checkpoint is unavailable")
	}

	if err = d.store.store(ctx, cp); err != nil {
		log.Errorw("storing checkpoint to disk", "err", err)
	}

	d.cancel()
	if err = d.sampler.wait(ctx); err != nil {
		return fmt.Errorf("DASer force quit: %w", err)
	}

	// save updated checkpoint after sampler and all workers are shut down
	if err = d.store.store(ctx, newCheckpoint(d.sampler.state.unsafeStats())); err != nil {
		log.Errorw("storing checkpoint to disk", "Err", err)
	}

	if err = d.store.wait(ctx); err != nil {
		return fmt.Errorf("DASer force quit with err: %w", err)
	}
	return d.subscriber.wait(ctx)
}

func (d *DASer) sample(ctx context.Context, h *header.ExtendedHeader) error {
	err := d.da.SharesAvailable(ctx, h.DAH)
	if err != nil {
		if err == context.Canceled {
			return err
		}
		var byzantineErr *share.ErrByzantine
		if errors.As(err, &byzantineErr) {
			log.Warn("Propagating proof...")
			sendErr := d.bcast.Broadcast(ctx, fraud.CreateBadEncodingProof(h.Hash(), uint64(h.Height), byzantineErr))
			if sendErr != nil {
				log.Errorw("fraud proof propagating failed", "err", sendErr)
			}
		}

		log.Errorw("sampling failed", "height", h.Height, "hash", h.Hash(),
			"square width", len(h.DAH.RowsRoots), "data root", h.DAH.Hash(), "err", err)
		return err
	}

	return nil
}

func (d *DASer) SamplingStats(ctx context.Context) (SamplingStats, error) {
	return d.sampler.stats(ctx)
}
