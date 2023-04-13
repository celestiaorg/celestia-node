package das

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

const (
	catchupJob jobType = "catchup"
	recentJob  jobType = "recent"
	retryJob   jobType = "retry"
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
	result

	curr uint64
}

type jobType string

// job represents headers interval to be processed by worker
type job struct {
	id      int
	jobType jobType
	from    uint64
	to      uint64

	// header is set only for recentJobs, avoiding an unnecessary call to the header store
	header *header.ExtendedHeader
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
			curr: j.from,
			result: result{
				job:    j,
				failed: make(map[uint64]int),
			},
		},
	}
}

func (w *worker) run(ctx context.Context, timeout time.Duration, resultCh chan<- result) {
	jobStart := time.Now()
	log.Debugw("start sampling worker", "from", w.state.from, "to", w.state.to)

	for curr := w.state.from; curr <= w.state.to; curr++ {
		err := w.sample(ctx, timeout, curr)
		w.setResult(curr, err)
		if errors.Is(err, context.Canceled) {
			// sampling worker will resume upon restart
			break
		}
	}

	log.With()
	log.Infow(
		"finished sampling headers",
		"from", w.state.from,
		"to", w.state.curr,
		"errors", len(w.state.failed),
		"finished (s)", time.Since(jobStart),
	)

	select {
	case resultCh <- w.state.result:
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
	w.metrics.observeSample(ctx, h, time.Since(start), w.state.jobType, err)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Debugw(
				"failed to sample header",
				"height", h.Height(),
				"hash", h.Hash(),
				"square width", len(h.DAH.RowsRoots),
				"data root", h.DAH.String(),
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
		"data root", h.DAH.String(),
		"finished (s)", time.Since(start),
	)

	// notify network about availability of new block data (note: only full nodes can notify)
	if w.state.job.jobType == recentJob {
		err = w.broadcast(ctx, shrexsub.Notification{
			DataHash: h.DataHash.Bytes(),
			Height:   uint64(h.Height()),
		})
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
		"data root", h.DAH.String(),
		"finished (s)", time.Since(start),
	)
	return h, nil
}

func (w *worker) setResult(curr uint64, err error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if err != nil {
		w.state.failed[curr]++
		w.state.err = errors.Join(w.state.err, fmt.Errorf("height: %d, err: %w", curr, err))
	}
	w.state.curr = curr
}

func (w *worker) getState() workerState {
	w.lock.Lock()
	defer w.lock.Unlock()
	return w.state
}
