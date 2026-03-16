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
	getter libhead.Store[*header.ExtendedHeader]      // retrieves past headers

	sampler    *samplingCoordinator
	store      checkpointStore
	subscriber subscriber

	cancel  context.CancelFunc
	running atomic.Bool
}

type (
	listenFn func(context.Context, *header.ExtendedHeader)
	sampleFn func(context.Context, *header.ExtendedHeader) error
)

// NewDASer creates a new DASer.
func NewDASer(
	da share.Availability,
	hsub libhead.Subscriber[*header.ExtendedHeader],
	getter libhead.Store[*header.ExtendedHeader],
	dstore datastore.Datastore,
	bcast fraud.Broadcaster[*header.ExtendedHeader],
	shrexBroadcast shrexsub.BroadcastFn,
	options ...Option,
) (*DASer, error) {
	d := &DASer{
		params:     DefaultParameters(),
		da:         da,
		bcast:      bcast,
		hsub:       hsub,
		getter:     getter,
		store:      newCheckpointStore(dstore),
		subscriber: newSubscriber(),
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

	cp, err := d.checkpoint(ctx)
	if err != nil {
		return err
	}

	sub, err := d.hsub.Subscribe()
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	go d.sampler.run(runCtx, cp)
	go d.subscriber.run(runCtx, sub, d.sampler.listen)
	go d.store.runBackgroundStore(runCtx, d.params.BackgroundStoreInterval, d.sampler.getCheckpoint)

	return nil
}

func (d *DASer) checkpoint(ctx context.Context) (checkpoint, error) {
	tail, err := d.getter.Tail(ctx)
	if err != nil {
		return checkpoint{}, err
	}

	head, err := d.getter.Head(ctx)
	if err != nil {
		return checkpoint{}, err
	}

	// load latest DASed checkpoint
	cp, err := d.store.load(ctx)
	switch {
	case errors.Is(err, datastore.ErrNotFound):
		log.Warnf("checkpoint not found, initializing with Tail (%d) and Head (%d)", tail.Height(), head.Height())

		cp = checkpoint{
			SampleFrom:  tail.Height(),
			NetworkHead: head.Height(),
		}
	case err != nil:
		return checkpoint{}, err
	default:
		if cp.SampleFrom < tail.Height() {
			cp.SampleFrom = tail.Height()
		}
		if cp.NetworkHead < head.Height() {
			cp.NetworkHead = head.Height()
		}

		for height := range cp.Failed {
			if height < tail.Height() {
				// means the sample status is outdated and we don't need to sample it again
				delete(cp.Failed, height)
			}
		}

		if len(cp.Workers) > 0 {
			wrkrs := make([]workerCheckpoint, 0, len(cp.Workers))
			for _, wrk := range cp.Workers {
				if wrk.To >= tail.Height() {
					if wrk.From < tail.Height() {
						wrk.From = tail.Height()
					}

					wrkrs = append(wrkrs, wrk)
				}
			}

			cp.Workers = wrkrs
		}
	}

	log.Info("starting DASer from checkpoint: ", cp.String())
	return cp, nil
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
