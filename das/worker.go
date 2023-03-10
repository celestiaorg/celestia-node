package das

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/multierr"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

type worker struct {
	lock  sync.Mutex
	state workerState

	getter    libhead.Getter[*header.ExtendedHeader]
	sampleFn  sampleFn
	broadcast shrexsub.BroadcastFn
	metrics   *metrics
}

// workerState contains important information about the state of a
// current sampling routine.
type workerState struct {
	job

	Curr   uint64
	Err    error
	failed []uint64
}

// job represents headers interval to be processed by worker
type job struct {
	id             int
	isRecentHeader bool
	header         *header.ExtendedHeader

	From uint64
	To   uint64
}

func newWorker(j job,
	getter libhead.Getter[*header.ExtendedHeader],
	sample sampleFn,
	broadcast shrexsub.BroadcastFn,
	metrics *metrics,
) worker {
	return worker{
		getter:    getter,
		sampleFn:  sample,
		broadcast: broadcast,
		metrics:   metrics,
		state: workerState{
			job:    j,
			Curr:   j.From,
			failed: make([]uint64, 0),
		},
	}
}

func (w *worker) run(ctx context.Context, timeout time.Duration, resultCh chan<- result) {
	jobStart := time.Now()
	log.Debugw("start sampling worker", "from", w.state.From, "to", w.state.To)

	for curr := w.state.From; curr <= w.state.To; curr++ {
		err := w.sample(ctx, timeout, curr)
		w.setResult(curr, err)
		if errors.Is(err, context.Canceled) {
			// sampling worker will resume upon restart
			break
		}
	}

	log.Infow(
		"finished sampling headers",
		"from", w.state.From,
		"to", w.state.Curr,
		"errors", len(w.state.failed),
		"finished (s)", time.Since(jobStart),
	)

	select {
	case resultCh <- result{
		job:    w.state.job,
		failed: w.state.failed,
		err:    w.state.Err,
	}:
	case <-ctx.Done():
	}
}

func (w *worker) sample(ctx context.Context, timeout time.Duration, height uint64) error {
	h, err := w.getHeader(ctx, height)
	if err != nil {
		return err
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err = w.sampleFn(ctx, h)
	w.metrics.observeSample(ctx, h, time.Since(start), err, w.state.isRecentHeader)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Debugw(
				"failed to sample header",
				"height", h.Height(),
				"hash", h.Hash(),
				"square width", len(h.DAH.RowsRoots),
				"data root", h.DAH.Hash(),
				"err", err,
				"finished (s)", time.Since(start),
			)
		}
		return err
	}

	log.Debugw(
		"sampled header",
		"height", h.Height(),
		"hash", h.Hash(),
		"square width", len(h.DAH.RowsRoots),
		"data root", h.DAH.Hash(),
		"finished (s)", time.Since(start),
	)

	// notify network about availability of new block data (note: only full nodes can notify)
	if w.state.isRecentHeader {
		err = w.broadcast(ctx, h.DataHash.Bytes())
		if err != nil {
			log.Warn("failed to broadcast availability message",
				"height", h.Height(), "hash", h.Hash(), "err", err)
		}
	}
	return nil
}

func (w *worker) getHeader(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	if w.state.header != nil {
		return w.state.header, nil
	}

	// TODO: get headers in batches
	start := time.Now()
	h, err := w.getter.GetByHeight(ctx, height)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Errorw("failed to get header from header store", "height", height,
				"finished (s)", time.Since(start))
		}
		return nil, err
	}

	w.metrics.observeGetHeader(ctx, time.Since(start))

	log.Debugw(
		"got header from header store",
		"height", h.Height(),
		"hash", h.Hash(),
		"square width", len(h.DAH.RowsRoots),
		"data root", h.DAH.Hash(),
		"finished (s)", time.Since(start),
	)
	return h, nil
}

func (w *worker) setResult(curr uint64, err error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if err != nil {
		w.state.failed = append(w.state.failed, curr)
		w.state.Err = multierr.Append(w.state.Err, fmt.Errorf("height: %v, err: %w", curr, err))
	}
	w.state.Curr = curr
}

func (w *worker) getState() workerState {
	w.lock.Lock()
	defer w.lock.Unlock()
	return w.state
}
