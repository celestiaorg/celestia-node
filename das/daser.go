package das

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/go-fraud"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
)

var log = logging.Logger("das")

// DASer continuously validates availability of data committed to headers.
type DASer struct {
	params Parameters

	da     share.Availability
	bcast  fraud.Broadcaster[*header.ExtendedHeader]
	hsub   libhead.Subscriber[*header.ExtendedHeader] // listens for new headers in the network
	getter libhead.Getter[*header.ExtendedHeader]     // retrieves past headers

	sampler    *samplingCoordinator
	store      checkpointStore
	subscriber subscriber

	cancel         context.CancelFunc
	subscriberDone chan struct{}
	running        atomic.Bool
}

type (
	listenFn func(context.Context, *header.ExtendedHeader)
	sampleFn func(context.Context, *header.ExtendedHeader) error
)

// NewDASer creates a new DASer.
func NewDASer(
	da share.Availability,
	hsub libhead.Subscriber[*header.ExtendedHeader],
	getter libhead.Getter[*header.ExtendedHeader],
	dstore datastore.Datastore,
	bcast fraud.Broadcaster[*header.ExtendedHeader],
	shrexBroadcast shrexsub.BroadcastFn,
	options ...Option,
) (*DASer, error) {
	d := &DASer{
		params:         DefaultParameters(),
		da:             da,
		bcast:          bcast,
		hsub:           hsub,
		getter:         getter,
		store:          newCheckpointStore(dstore),
		subscriber:     newSubscriber(),
		subscriberDone: make(chan struct{}),
	}

	for _, applyOpt := range options {
		applyOpt(d)
	}

	err := d.params.Validate()
	if err != nil {
		return nil, err
	}

	d.sampler = newSamplingCoordinator(d.params, getter, d.sample, shrexBroadcast)
	return d, nil
}

// Start initiates subscription for new ExtendedHeaders and spawns a sampling routine.
func (d *DASer) Start(ctx context.Context) error {
	if !d.running.CompareAndSwap(false, true) {
		return errors.New("da: DASer already started")
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
			SampleFrom:  d.params.SampleFrom,
			NetworkHead: d.params.SampleFrom,
		}

		// attempt to get head info. No need to handle error, later DASer
		// will be able to find new head from subscriber after it is started
		if h, err := d.getter.Head(ctx); err == nil {
			cp.NetworkHead = h.Height()
		}
	}
	log.Info("starting DASer from checkpoint: ", cp.String())

	runCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	go d.sampler.run(runCtx, cp)
	go d.subscriber.run(runCtx, sub, d.sampler.listen)
	go d.store.runBackgroundStore(runCtx, d.params.BackgroundStoreInterval, d.sampler.getCheckpoint)

	return nil
}

// Stop stops sampling.
func (d *DASer) Stop(ctx context.Context) error {
	if !d.running.CompareAndSwap(true, false) {
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

	if err := d.sampler.metrics.close(); err != nil {
		log.Warnw("closing metrics", "err", err)
	}

	if err = d.sampler.wait(ctx); err != nil {
		return fmt.Errorf("DASer force quit: %w", err)
	}

	// save updated checkpoint after sampler and all workers are shut down
	if err = d.store.store(ctx, newCheckpoint(d.sampler.state.unsafeStats())); err != nil {
		log.Errorw("storing checkpoint to disk", "err", err)
	}

	if err = d.store.wait(ctx); err != nil {
		return fmt.Errorf("DASer force quit with err: %w", err)
	}
	return d.subscriber.wait(ctx)
}

func (d *DASer) sample(ctx context.Context, h *header.ExtendedHeader) error {
	err := d.da.SharesAvailable(ctx, h)
	if err != nil {
		var byzantineErr *byzantine.ErrByzantine
		if errors.As(err, &byzantineErr) {
			log.Warn("Propagating proof...")
			sendErr := d.bcast.Broadcast(ctx, byzantine.CreateBadEncodingProof(h.Hash(), h.Height(), byzantineErr))
			if sendErr != nil {
				log.Errorw("fraud proof propagating failed", "err", sendErr)
			}
		}
		return err
	}
	return nil
}

// SamplingStats returns the current statistics over the DA sampling process.
func (d *DASer) SamplingStats(ctx context.Context) (SamplingStats, error) {
	return d.sampler.stats(ctx)
}

// WaitCatchUp waits for DASer to indicate catchup is done
func (d *DASer) WaitCatchUp(ctx context.Context) error {
	return d.sampler.state.waitCatchUp(ctx)
}
