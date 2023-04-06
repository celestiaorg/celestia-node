package das

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

func TestCoordinator(t *testing.T) {
	t.Run("test run", func(t *testing.T) {
		testParams := defaultTestParams()

		ctx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)
		sampler := newMockSampler(testParams.sampleFrom, testParams.networkHead)
		coordinator := newSamplingCoordinator(testParams.dasParams, getterStub{}, onceMiddleWare(sampler.sample), nil)

		go coordinator.run(ctx, sampler.checkpoint)

		// check if all jobs were sampled successfully
		assert.NoError(t, sampler.finished(ctx), "not all headers were sampled")

		// wait for coordinator to indicateDone catchup
		assert.NoError(t, coordinator.state.waitCatchUp(ctx))
		assert.Emptyf(t, coordinator.state.failed, "failed list should be empty")

		cancel()
		stopCtx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)
		defer cancel()
		assert.NoError(t, coordinator.wait(stopCtx))
		assert.Equal(t, sampler.finalState(), newCheckpoint(coordinator.state.unsafeStats()))
	})

	t.Run("discovered new headers", func(t *testing.T) {
		testParams := defaultTestParams()

		ctx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)

		sampler := newMockSampler(testParams.sampleFrom, testParams.networkHead)

		newhead := testParams.networkHead + 200
		coordinator := newSamplingCoordinator(testParams.dasParams, getterStub{}, sampler.sample, newBroadcastMock(1))
		go coordinator.run(ctx, sampler.checkpoint)

		// discover new height
		sampler.discover(ctx, newhead, coordinator.listen)

		// check if all jobs were sampled successfully
		assert.NoError(t, sampler.finished(ctx), "not all headers were sampled")

		// wait for coordinator to indicateDone catchup
		assert.NoError(t, coordinator.state.waitCatchUp(ctx))
		assert.Emptyf(t, coordinator.state.failed, "failed list should be empty")

		cancel()
		stopCtx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)
		defer cancel()
		assert.NoError(t, coordinator.wait(stopCtx))
		assert.Equal(t, sampler.finalState(), newCheckpoint(coordinator.state.unsafeStats()))
	})

	t.Run("prioritize newly discovered over known", func(t *testing.T) {
		testParams := defaultTestParams()

		testParams.dasParams.ConcurrencyLimit = 1
		testParams.dasParams.SamplingRange = 4

		testParams.sampleFrom = 1
		testParams.networkHead = 10
		toBeDiscovered := uint64(20)

		sampler := newMockSampler(testParams.sampleFrom, testParams.networkHead)

		ctx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)

		// lock worker before start, to not let it indicateDone before discover
		lk := newLock(testParams.sampleFrom, testParams.sampleFrom)

		order := newCheckOrder().addInterval(toBeDiscovered, toBeDiscovered)

		// expect worker to prioritize newly discovered
		order.addInterval(
			testParams.sampleFrom,
			toBeDiscovered,
		)

		// start coordinator
		coordinator := newSamplingCoordinator(testParams.dasParams, getterStub{},
			lk.middleWare(
				order.middleWare(sampler.sample),
			),
			newBroadcastMock(1),
		)
		go coordinator.run(ctx, sampler.checkpoint)

		// discover new height
		sampler.discover(ctx, toBeDiscovered, coordinator.listen)

		// check if no header were sampled yet
		for sampler.sampledAmount() != 1 {
			time.Sleep(time.Millisecond)
			select {
			case <-ctx.Done():
				assert.NoError(t, ctx.Err())
			default:
			}
		}

		// unblock worker
		lk.release(testParams.sampleFrom)

		// check if all jobs were sampled successfully
		assert.NoError(t, sampler.finished(ctx), "not all headers were sampled")

		// wait for coordinator to indicateDone catchup
		assert.NoError(t, coordinator.state.waitCatchUp(ctx))
		assert.Emptyf(t, coordinator.state.failed, "failed list should be empty")

		cancel()
		stopCtx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)
		defer cancel()
		assert.NoError(t, coordinator.wait(stopCtx))
		assert.Equal(t, sampler.finalState(), newCheckpoint(coordinator.state.unsafeStats()))
	})

	t.Run("recent headers sampling routine should not lock other workers", func(t *testing.T) {
		testParams := defaultTestParams()

		testParams.networkHead = uint64(20)
		ctx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)

		sampler := newMockSampler(testParams.sampleFrom, testParams.networkHead)

		lk := newLock(testParams.sampleFrom, testParams.networkHead) // lock all workers before start
		coordinator := newSamplingCoordinator(testParams.dasParams, getterStub{},
			lk.middleWare(sampler.sample), newBroadcastMock(1))
		go coordinator.run(ctx, sampler.checkpoint)

		// discover new height and lock it
		discovered := testParams.networkHead + 1
		lk.add(discovered)
		sampler.discover(ctx, discovered, coordinator.listen)

		// check if no header were sampled yet
		assert.Equal(t, 0, sampler.sampledAmount())

		// unblock workers to resume sampling
		lk.releaseAll(discovered)

		// wait for coordinator to run sample on all headers except discovered
		time.Sleep(100 * time.Millisecond)

		// check that only last header is pending
		assert.EqualValues(t, int(discovered-testParams.sampleFrom), sampler.doneAmount())
		assert.False(t, sampler.heightIsDone(discovered))

		// release all headers for coordinator
		lk.releaseAll()

		// check if all jobs were sampled successfully
		assert.NoError(t, sampler.finished(ctx), "not all headers were sampled")

		// wait for coordinator to indicateDone catchup
		assert.NoError(t, coordinator.state.waitCatchUp(ctx))
		assert.Emptyf(t, coordinator.state.failed, "failed list is not empty")

		cancel()
		stopCtx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)
		defer cancel()
		assert.NoError(t, coordinator.wait(stopCtx))
		assert.Equal(t, sampler.finalState(), newCheckpoint(coordinator.state.unsafeStats()))
	})

	t.Run("failed should be stored", func(t *testing.T) {
		testParams := defaultTestParams()
		testParams.sampleFrom = 1
		ctx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)

		bornToFail := []uint64{4, 8, 15, 16, 23, 42}
		sampler := newMockSampler(testParams.sampleFrom, testParams.networkHead, bornToFail...)

		coordinator := newSamplingCoordinator(
			testParams.dasParams,
			getterStub{},
			onceMiddleWare(sampler.sample),
			newBroadcastMock(1),
		)
		go coordinator.run(ctx, sampler.checkpoint)

		// wait for coordinator to go over all headers
		assert.NoError(t, sampler.finished(ctx))

		cancel()
		stopCtx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)
		defer cancel()
		assert.NoError(t, coordinator.wait(stopCtx))

		// failed item should be either in failed map or be processed by worker
		cp := newCheckpoint(coordinator.state.unsafeStats())
		for _, failedHeight := range bornToFail {
			if _, ok := cp.Failed[failedHeight]; ok {
				continue
			}
			for _, w := range cp.Workers {
				if w.JobType == retryJob && w.From == failedHeight {
					continue
				}
			}
			t.Error("header is not found neither in failed nor in workers")
		}
	})

	t.Run("failed should retry on restart", func(t *testing.T) {
		testParams := defaultTestParams()

		testParams.sampleFrom = uint64(50)
		ctx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)

		failedLastRun := map[uint64]int{4: 1, 8: 2, 15: 1, 16: 1, 23: 1, 42: 1, testParams.sampleFrom - 1: 1}

		sampler := newMockSampler(testParams.sampleFrom, testParams.networkHead)
		sampler.checkpoint.Failed = failedLastRun

		coordinator := newSamplingCoordinator(
			testParams.dasParams,
			getterStub{},
			onceMiddleWare(sampler.sample),
			newBroadcastMock(1),
		)
		go coordinator.run(ctx, sampler.checkpoint)

		// check if all jobs were sampled successfully
		assert.NoError(t, sampler.finished(ctx), "not all headers were sampled")

		// wait for coordinator to indicateDone catchup
		assert.NoError(t, coordinator.state.waitCatchUp(ctx))

		cancel()
		stopCtx, cancel := context.WithTimeout(context.Background(), testParams.timeoutDelay)
		defer cancel()
		assert.NoError(t, coordinator.wait(stopCtx))

		expectedState := sampler.finalState()
		expectedState.Failed = make(map[uint64]int)
		assert.Equal(t, expectedState, newCheckpoint(coordinator.state.unsafeStats()))
	})
}

func BenchmarkCoordinator(b *testing.B) {
	timeoutDelay := 5 * time.Second

	params := DefaultParameters()
	params.SamplingRange = 10
	params.ConcurrencyLimit = 100

	b.Run("bench run", func(b *testing.B) {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutDelay)
		coordinator := newSamplingCoordinator(
			params,
			newBenchGetter(),
			func(ctx context.Context, h *header.ExtendedHeader) error { return nil },
			newBroadcastMock(1),
		)
		go coordinator.run(ctx, checkpoint{
			SampleFrom:  1,
			NetworkHead: uint64(b.N),
		})

		// wait for coordinator to indicateDone catchup
		if err := coordinator.state.waitCatchUp(ctx); err != nil {
			b.Error(err)
		}
		cancel()
	})
}

// ensures all headers are sampled in range except ones that are born to fail
type mockSampler struct {
	lock sync.Mutex

	checkpoint
	bornToFail map[uint64]bool
	done       map[uint64]int

	isFinished bool
	finishedCh chan struct{}
}

func newMockSampler(sampledBefore, sampleTo uint64, bornToFail ...uint64) mockSampler {
	failMap := make(map[uint64]bool)
	for _, h := range bornToFail {
		failMap[h] = true
	}
	return mockSampler{
		checkpoint: checkpoint{
			SampleFrom:  sampledBefore,
			NetworkHead: sampleTo,
			Failed:      make(map[uint64]int),
			Workers:     make([]workerCheckpoint, 0),
		},
		bornToFail: failMap,
		done:       make(map[uint64]int),
		finishedCh: make(chan struct{}),
	}
}

func (m *mockSampler) sample(ctx context.Context, h *header.ExtendedHeader) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	height := uint64(h.Height())
	m.done[height]++

	if len(m.done) > int(m.NetworkHead-m.SampleFrom) && !m.isFinished {
		m.isFinished = true
		close(m.finishedCh)
	}

	if m.bornToFail[height] {
		return errors.New("born to fail, sad life")
	}

	if height > m.NetworkHead || height < m.SampleFrom {
		if _, ok := m.checkpoint.Failed[height]; !ok {
			return fmt.Errorf("header: %v out of range: %v-%v", h, m.SampleFrom, m.NetworkHead)
		}
	}
	return nil
}

// finished returns when all jobs were sampled successfully
func (m *mockSampler) finished(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.finishedCh:
	}
	return nil
}

func (m *mockSampler) heightIsDone(h uint64) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.done[h] != 0
}

func (m *mockSampler) doneAmount() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.done)
}

func (m *mockSampler) finalState() checkpoint {
	m.lock.Lock()
	defer m.lock.Unlock()

	finalState := m.checkpoint
	finalState.SampleFrom = finalState.NetworkHead + 1
	return finalState
}

func (m *mockSampler) discover(ctx context.Context, newHeight uint64, emit listenFn) {
	m.lock.Lock()

	if newHeight > m.checkpoint.NetworkHead {
		m.checkpoint.NetworkHead = newHeight
		if m.isFinished {
			m.finishedCh = make(chan struct{})
			m.isFinished = false
		}
	}
	m.lock.Unlock()
	emit(ctx, &header.ExtendedHeader{
		Commit:    &types.Commit{},
		RawHeader: header.RawHeader{Height: int64(newHeight)},
		DAH:       &header.DataAvailabilityHeader{RowsRoots: make([][]byte, 0)},
	})
}

func (m *mockSampler) sampledAmount() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.done)
}

// ensures correct order of operations
type checkOrder struct {
	lock  sync.Mutex
	queue []uint64
}

func newCheckOrder() *checkOrder {
	return &checkOrder{}
}

func (o *checkOrder) addInterval(start, end uint64) *checkOrder {
	o.lock.Lock()
	defer o.lock.Unlock()

	if end > start {
		for end >= start {
			o.queue = append(o.queue, start)
			start++
		}
		return o
	}

	for start >= end {
		o.queue = append(o.queue, start)
		if start == 0 {
			return o
		}
		start--

	}
	return o
}

// splits interval into ranges with stackSize length and puts them with reverse order
func (o *checkOrder) addStacks(start, end, stackSize uint64) uint64 {
	if start+stackSize-1 < end {
		end = o.addStacks(start+stackSize, end, stackSize)
	}
	if start > end {
		start = end
	}
	o.addInterval(start, end)
	return start - 1
}

func TestOrder(t *testing.T) {
	o := newCheckOrder().addInterval(0, 3).addInterval(3, 0)
	assert.Equal(t, []uint64{0, 1, 2, 3, 3, 2, 1, 0}, o.queue)
}

func TestStack(t *testing.T) {
	o := newCheckOrder()
	o.addStacks(10, 20, 3)
	assert.Equal(t, []uint64{19, 20, 16, 17, 18, 13, 14, 15, 10, 11, 12}, o.queue)
}

func (o *checkOrder) middleWare(out sampleFn) sampleFn {
	return func(ctx context.Context, h *header.ExtendedHeader) error {
		o.lock.Lock()

		if len(o.queue) > 0 {
			// check last item in queue to be same as input
			if o.queue[0] != uint64(h.Height()) {
				defer o.lock.Unlock()
				return fmt.Errorf("expected height: %v,got: %v", o.queue[0], h.Height())
			}
			o.queue = o.queue[1:]
		}

		o.lock.Unlock()
		return out(ctx, h)
	}
}

// blocks operations if item is in lock list
type lock struct {
	m         sync.Mutex
	blockList map[uint64]chan struct{}
}

func newLock(from, to uint64) *lock {
	list := make(map[uint64]chan struct{})
	for from <= to {
		list[from] = make(chan struct{})
		from++
	}
	return &lock{
		blockList: list,
	}
}

func (l *lock) add(hs ...uint64) {
	l.m.Lock()
	defer l.m.Unlock()
	for _, h := range hs {
		l.blockList[h] = make(chan struct{})
	}
}

func (l *lock) release(hs ...uint64) {
	l.m.Lock()
	defer l.m.Unlock()

	for _, h := range hs {
		if ch, ok := l.blockList[h]; ok {
			close(ch)
			delete(l.blockList, h)
		}
	}
}

func (l *lock) releaseAll(except ...uint64) {
	m := make(map[uint64]bool)
	for _, h := range except {
		m[h] = true
	}

	l.m.Lock()
	defer l.m.Unlock()

	for h, ch := range l.blockList {
		if m[h] {
			continue
		}
		close(ch)
		delete(l.blockList, h)
	}
}

func (l *lock) middleWare(out sampleFn) sampleFn {
	return func(ctx context.Context, h *header.ExtendedHeader) error {
		l.m.Lock()
		ch, blocked := l.blockList[uint64(h.Height())]
		l.m.Unlock()
		if !blocked {
			return out(ctx, h)
		}

		select {
		case <-ch:
			return out(ctx, h)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func onceMiddleWare(out sampleFn) sampleFn {
	db := make(map[int64]int)
	m := sync.Mutex{}
	return func(ctx context.Context, h *header.ExtendedHeader) error {
		m.Lock()
		db[h.Height()]++
		if db[h.Height()] > 1 {
			m.Unlock()
			return fmt.Errorf("header sampled more than once: %v", h.Height())
		}
		m.Unlock()
		return out(ctx, h)
	}
}

type testParams struct {
	networkHead  uint64
	sampleFrom   uint64
	timeoutDelay time.Duration
	dasParams    Parameters
}

func defaultTestParams() testParams {
	dasParamsDefault := DefaultParameters()
	return testParams{
		networkHead:  uint64(500),
		sampleFrom:   dasParamsDefault.SampleFrom,
		timeoutDelay: 5 * time.Second,
		dasParams:    dasParamsDefault,
	}
}

func newBroadcastMock(callLimit int) shrexsub.BroadcastFn {
	var m sync.Mutex
	return func(ctx context.Context, hash shrexsub.Notification) error {
		m.Lock()
		defer m.Unlock()
		if callLimit == 0 {
			return errors.New("exceeded mock call limit")
		}
		callLimit--
		return nil
	}
}
