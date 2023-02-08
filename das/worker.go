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
)

type worker struct {
	lock  sync.Mutex
	state workerState
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
	id   int
	From uint64
	To   uint64
}

func (w *worker) run(
	ctx context.Context,
	getter libhead.Getter[*header.ExtendedHeader],
	sample sampleFn,
	metrics *metrics,
	resultCh chan<- result) {
	jobStart := time.Now()
	log.Debugw("start sampling worker", "from", w.state.From, "to", w.state.To)

	for curr := w.state.From; curr <= w.state.To; curr++ {
		startGet := time.Now()
		// TODO: get headers in batches
		h, err := getter.GetByHeight(ctx, curr)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				// sampling worker will resume upon restart
				break
			}
			w.setResult(curr, err)
			log.Errorw("failed to get header from header store", "height", curr,
				"finished (s)", time.Since(startGet))
			continue
		}

		metrics.observeGetHeader(ctx, time.Since(startGet))

		log.Debugw(
			"got header from header store",
			"height", h.Height(),
			"hash", h.Hash(),
			"square width", len(h.DAH.RowsRoots),
			"data root", h.DAH.Hash(),
			"finished (s)", time.Since(startGet),
		)

		startSample := time.Now()
		err = sample(ctx, h)
		if errors.Is(err, context.Canceled) {
			// sampling worker will resume upon restart
			break
		}
		w.setResult(curr, err)
		metrics.observeSample(ctx, h, time.Since(startSample), err)
		if err != nil {
			log.Debugw(
				"failed to sampled header",
				"height", h.Height(),
				"hash", h.Hash(),
				"square width", len(h.DAH.RowsRoots),
				"data root", h.DAH.Hash(),
				"err", err,
			)
		} else {
			log.Debugw(
				"sampled header",
				"height", h.Height(),
				"hash", h.Hash(),
				"square width", len(h.DAH.RowsRoots),
				"data root", h.DAH.Hash(),
				"finished (s)", time.Since(startSample),
			)
		}
	}

	if w.state.Curr > w.state.From {
		jobTime := time.Since(jobStart)
		log.Infow(
			"sampled headers",
			"from", w.state.From,
			"to", w.state.Curr,
			"finished (s)",
			jobTime.Seconds(),
		)
	}

	select {
	case resultCh <- result{
		job:    w.state.job,
		failed: w.state.failed,
		err:    w.state.Err,
	}:
	case <-ctx.Done():
	}
}

func newWorker(j job) worker {
	return worker{
		state: workerState{
			job:    j,
			Curr:   j.From,
			failed: make([]uint64, 0),
		},
	}
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
