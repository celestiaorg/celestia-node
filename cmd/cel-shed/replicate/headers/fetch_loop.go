package headers

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	libhead_store "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
)

// chunk is a half-open description of [start, end] inclusive heights and the
// chunk index in dispatch order.
type chunk struct {
	index uint64
	start uint64
	end   uint64
}

type chunkResult struct {
	chunk   chunk
	headers []*header.ExtendedHeader
	err     error
}

type replicationState struct {
	ctx    context.Context
	cancel context.CancelFunc

	exchange exchanger
	hstore   *libhead_store.Store[*header.ExtendedHeader]
	prog     *Progress
	chainID  string

	startHeight    uint64
	targetHeight   uint64
	nextHeight     uint64
	totalChunks    uint64
	concurrency    int
	requestTimeout time.Duration

	nextChunk atomic.Uint64
	results   chan chunkResult
	wg        sync.WaitGroup

	lastAppended *header.ExtendedHeader
}

// replicateHeaderRange dispatches concurrent chunk workers and writes their
// results in strict dispatch order. The returned header is the last one that
// was successfully appended (nil if none). On error, the caller can still
// publish that header as the partial head.
func replicateHeaderRange(
	ctx context.Context,
	exchange exchanger,
	hstore *libhead_store.Store[*header.ExtendedHeader],
	startHeight, targetHeight uint64,
	concurrency int,
	requestTimeout time.Duration,
	chainID string,
	prog *Progress,
) (*header.ExtendedHeader, error) {
	if startHeight > targetHeight {
		return nil, nil
	}
	if concurrency < 1 {
		concurrency = 1
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	r := &replicationState{
		ctx:            workerCtx,
		cancel:         cancel,
		exchange:       exchange,
		hstore:         hstore,
		prog:           prog,
		chainID:        chainID,
		startHeight:    startHeight,
		targetHeight:   targetHeight,
		concurrency:    concurrency,
		requestTimeout: requestTimeout,
		results:        make(chan chunkResult, concurrency),
	}

	if err := r.prepareStart(); err != nil {
		return r.lastAppended, err
	}
	if r.nextHeight > r.targetHeight {
		return r.lastAppended, nil
	}

	r.totalChunks = countChunks(r.nextHeight, r.targetHeight, maxRangeRequest)
	r.startWorkers()

	if err := r.appendResultsInOrder(); err != nil {
		return r.lastAppended, err
	}
	if err := r.validateComplete(); err != nil {
		return r.lastAppended, err
	}
	return r.lastAppended, nil
}

// prepareStart establishes the local trusted tip. Height 1 has no parent so
// it is fetched and appended on its own; for higher starts the previous
// height is loaded from the local store.
func (r *replicationState) prepareStart() error {
	if r.startHeight == 1 {
		h, err := getByHeightWithRetry(r.ctx, r.exchange, 1, r.requestTimeout)
		if err != nil {
			return err
		}
		if err := validateChunkChainID([]*header.ExtendedHeader{h}, r.chainID); err != nil {
			return err
		}
		if err := r.appendHeaders([]*header.ExtendedHeader{h}); err != nil {
			return err
		}
		r.nextHeight = 2
		return nil
	}

	localAnchor, err := r.hstore.GetByHeight(r.ctx, r.startHeight-1)
	if err != nil {
		return fmt.Errorf("read local anchor at height %d: %w", r.startHeight-1, err)
	}
	r.lastAppended = localAnchor
	r.nextHeight = r.startHeight
	return nil
}

func (r *replicationState) startWorkers() {
	r.wg.Add(r.concurrency)
	for i := 0; i < r.concurrency; i++ {
		go r.runWorker()
	}
	go func() {
		r.wg.Wait()
		close(r.results)
	}()
}

func (r *replicationState) runWorker() {
	defer r.wg.Done()

	for {
		idx := r.nextChunk.Add(1) - 1
		if idx >= r.totalChunks {
			return
		}

		c := chunkByIndex(r.nextHeight, r.targetHeight, maxRangeRequest, idx)
		hdrs, err := fetchChunkWithRetry(r.ctx, r.exchange, c, r.requestTimeout)

		if !r.sendResult(chunkResult{chunk: c, headers: hdrs, err: err}) {
			return
		}
		if err != nil {
			r.cancel()
			return
		}
	}
}

func (r *replicationState) sendResult(res chunkResult) bool {
	select {
	case r.results <- res:
		return true
	case <-r.ctx.Done():
		return false
	}
}

// appendResultsInOrder drains the workers' result channel, buffering
// out-of-order chunks and appending only the next expected index.
func (r *replicationState) appendResultsInOrder() error {
	pending := make(map[uint64]chunkResult)
	var wantIndex uint64
	wantStart := r.nextHeight

	for res := range r.results {
		if res.err != nil {
			r.cancel()
			return res.err
		}

		pending[res.chunk.index] = res

		for {
			nextRes, ok := pending[wantIndex]
			if !ok {
				break
			}
			delete(pending, wantIndex)

			if err := r.appendReadyChunk(nextRes, wantStart); err != nil {
				return err
			}

			wantStart = nextRes.chunk.end + 1
			wantIndex++
		}
	}

	if wantIndex != r.totalChunks {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		return fmt.Errorf("replication stopped after %d/%d chunks", wantIndex, r.totalChunks)
	}
	if len(pending) != 0 {
		return fmt.Errorf("internal error: %d chunks left pending", len(pending))
	}
	return nil
}

func (r *replicationState) appendReadyChunk(res chunkResult, wantStart uint64) error {
	if err := validateFetchedChunk(res.chunk, res.headers, wantStart); err != nil {
		r.cancel()
		return err
	}
	if err := validateChunkChainID(res.headers, r.chainID); err != nil {
		r.cancel()
		return err
	}
	if err := verifyChunkAgainstLocalTip(r.lastAppended, res.headers); err != nil {
		r.cancel()
		return err
	}
	if err := r.appendHeaders(res.headers); err != nil {
		r.cancel()
		return err
	}
	return nil
}

func (r *replicationState) appendHeaders(hdrs []*header.ExtendedHeader) error {
	if len(hdrs) == 0 {
		return fmt.Errorf("append empty header batch")
	}
	if err := r.hstore.Append(r.ctx, hdrs...); err != nil {
		return fmt.Errorf("append headers from %d n=%d: %w",
			hdrs[0].Height(), len(hdrs), err)
	}
	r.lastAppended = hdrs[len(hdrs)-1]
	if r.prog != nil {
		r.prog.AddStored(uint64(len(hdrs)))
	}
	return nil
}

func (r *replicationState) validateComplete() error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if r.lastAppended == nil || r.lastAppended.Height() != r.targetHeight {
		got := uint64(0)
		if r.lastAppended != nil {
			got = r.lastAppended.Height()
		}
		return fmt.Errorf("replication ended at height %d, expected %d",
			got, r.targetHeight)
	}
	return nil
}

func countChunks(start, target, chunkSize uint64) uint64 {
	totalHeaders := target - start + 1
	return (totalHeaders + chunkSize - 1) / chunkSize
}

func chunkByIndex(start, target, chunkSize, index uint64) chunk {
	cs := start + index*chunkSize
	ce := cs + chunkSize - 1
	if ce > target {
		ce = target
	}
	return chunk{index: index, start: cs, end: ce}
}
