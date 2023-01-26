package das

import (
	"context"
	"sync"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
)

// samplingCoordinator runs and coordinates sampling workers and updates current sampling state
type samplingCoordinator struct {
	concurrencyLimit int

	getter   libhead.Getter[*header.ExtendedHeader]
	sampleFn sampleFn

	state coordinatorState

	// resultCh fans-in sampling results from worker to coordinator
	resultCh chan result
	// updHeadCh signals to update network head header height
	updHeadCh chan uint64
	// waitCh signals to block coordinator for external access to state
	waitCh chan *sync.WaitGroup

	workersWg sync.WaitGroup
	metrics   *metrics
	done
}

// result will carry errors to coordinator after worker finishes the job
type result struct {
	job
	failed []uint64
	err    error
}

func newSamplingCoordinator(
	params Parameters,
	getter libhead.Getter[*header.ExtendedHeader],
	sample sampleFn,
) *samplingCoordinator {
	return &samplingCoordinator{
		concurrencyLimit: params.ConcurrencyLimit,
		getter:           getter,
		sampleFn:         sample,
		state:            newCoordinatorState(params),
		resultCh:         make(chan result),
		updHeadCh:        make(chan uint64),
		waitCh:           make(chan *sync.WaitGroup),
		done:             newDone("sampling coordinator"),
	}
}

func (sc *samplingCoordinator) run(ctx context.Context, cp checkpoint) {
	sc.state.resumeFromCheckpoint(cp)

	// the amount of sampled headers from the last checkpoint
	totalSampledFromCheckpoint := int64(cp.TotalSampled())
	sc.metrics.recordTotalSampled(ctx, totalSampledFromCheckpoint)

	// resume workers
	for _, wk := range cp.Workers {
		sc.runWorker(ctx, sc.state.newJob(wk.From, wk.To))
	}

	for {
		for !sc.concurrencyLimitReached() {
			next, found := sc.state.nextJob()
			if !found {
				break
			}
			sc.runWorker(ctx, next)
		}

		select {
		case head := <-sc.updHeadCh:
			if sc.state.updateHead(head) {
				sc.metrics.observeNewHead(ctx)
			}
		case res := <-sc.resultCh:
			sc.state.handleResult(res)

			// totalSampledFromWorker is the amount of successfully
			// sampled headers from the worker
			totalSampledFromWorker := int64(res.To-res.From) - int64(len(res.failed))
			sc.metrics.recordTotalSampled(ctx, totalSampledFromWorker)

		case wg := <-sc.waitCh:
			wg.Wait()
		case <-ctx.Done():
			sc.workersWg.Wait()
			sc.indicateDone()
			return
		}
	}
}

// runWorker runs job in separate worker go-routine
func (sc *samplingCoordinator) runWorker(ctx context.Context, j job) {
	w := newWorker(j)
	sc.state.putInProgress(j.id, w.getState)

	// launch worker go-routine
	sc.workersWg.Add(1)
	go func() {
		defer sc.workersWg.Done()
		w.run(ctx, sc.getter, sc.sampleFn, sc.metrics, sc.resultCh)
	}()
}

// listen notifies the coordinator about a new network head received via subscription.
func (sc *samplingCoordinator) listen(ctx context.Context, height uint64) {
	select {
	case sc.updHeadCh <- height:
	case <-ctx.Done():
	}
}

// stats pauses the coordinator to get stats in a concurrently safe manner
func (sc *samplingCoordinator) stats(ctx context.Context) (SamplingStats, error) {
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Done()

	select {
	case sc.waitCh <- &wg:
	case <-ctx.Done():
		return SamplingStats{}, ctx.Err()
	}

	return sc.state.unsafeStats(), nil
}

func (sc *samplingCoordinator) getCheckpoint(ctx context.Context) (checkpoint, error) {
	stats, err := sc.stats(ctx)
	if err != nil {
		return checkpoint{}, err
	}
	return newCheckpoint(stats), nil
}

// concurrencyLimitReached indicates whether concurrencyLimit has been reached
func (sc *samplingCoordinator) concurrencyLimitReached() bool {
	return len(sc.state.inProgress) >= sc.concurrencyLimit
}
