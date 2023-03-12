package sync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/header/test"
	"github.com/stretchr/testify/assert"
)

func TestSyncExchangeHead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fex := &fakeExchange[*test.DummyHeader]{}
	sex := syncExchange[*test.DummyHeader]{Exchange: fex}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h, err := sex.Head(ctx)
			if h != nil || err != errFakeHead {
				t.Fail()
			}
		}()
	}
	wg.Wait()

	assert.EqualValues(t, 1, fex.hits.Load())
}

var errFakeHead = fmt.Errorf("head")

type fakeExchange[H header.Header] struct {
	hits atomic.Uint32
}

func (f *fakeExchange[H]) Head(ctx context.Context) (h H, err error) {
	f.hits.Add(1)
	select {
	case <-time.After(time.Millisecond*100):
		err = errFakeHead
	case <-ctx.Done():
		err = ctx.Err()
	}

	return
}

func (f *fakeExchange[H]) Get(ctx context.Context, hash header.Hash) (H, error) {
	panic("implement me")
}

func (f *fakeExchange[H]) GetByHeight(ctx context.Context, u uint64) (H, error) {
	panic("implement me")
}

func (f *fakeExchange[H]) GetRangeByHeight(ctx context.Context, from, amount uint64) ([]H, error) {
	panic("implement me")
}

func (f *fakeExchange[H]) GetVerifiedRange(ctx context.Context, from H, amount uint64) ([]H, error) {
	panic("implement me")
}

