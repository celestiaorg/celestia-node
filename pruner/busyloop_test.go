package pruner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

type alwaysFailPruner struct{ calls atomic.Int64 }

func (p *alwaysFailPruner) Prune(context.Context, *header.ExtendedHeader) error {
	p.calls.Add(1)
	return errors.New("permission denied")
}

// TestPruneSpinsWhenWholeBatchFails shows that a single prune round never returns
// (and keeps checkpointMu locked) when every header of a full batch fails to prune:
// lastPrunedHeader does not advance, len(headers) == maxHeadersPerLoop, so the
// loop re-fetches and re-fails the same batch forever instead of waiting for the
// next pruneCycle.
func TestPruneSpinsWhenWholeBatchFails(t *testing.T) {
	old := maxHeadersPerLoop
	maxHeadersPerLoop = 8
	t.Cleanup(func() { maxHeadersPerLoop = old })

	blockTime := time.Minute
	suite := headertest.NewTestSuite(t,
		headertest.WithValidators(1),
		headertest.WithStartTime(time.Now().Add(-100*blockTime)),
		headertest.WithBlockTime(blockTime),
	)
	hs := headertest.NewCustomStore(t, suite, 100)

	fp := &alwaysFailPruner{}
	serv, err := NewService(fp, 10*blockTime, hs, sync.MutexWrap(datastore.NewMapDatastore()), blockTime)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	serv.ctx, serv.cancel = ctx, cancel
	require.NoError(t, serv.loadCheckpoint(ctx))

	done := make(chan struct{})
	go func() {
		serv.prune(ctx)
		close(done)
	}()

	select {
	case <-done:
		// expected behavior: a failed batch ends the round; failures are retried next cycle
	case <-time.After(2 * time.Second):
		lockFree := serv.checkpointMu.TryLock()
		if lockFree {
			serv.checkpointMu.Unlock()
		}
		cancel()
		<-done
		t.Fatalf("prune round did not return after 2s: %d Prune calls for %d distinct headers, checkpointMu free=%v",
			fp.calls.Load(), maxHeadersPerLoop, lockFree)
	}
}
