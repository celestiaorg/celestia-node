package das

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share/availability/mocks"
)

// chanSubscriber delivers headers pushed into ch.
type chanSubscriber struct {
	ch chan *header.ExtendedHeader
}

func (s *chanSubscriber) Subscribe() (libhead.Subscription[*header.ExtendedHeader], error) {
	return s, nil
}

func (s *chanSubscriber) SetVerifier(func(context.Context, *header.ExtendedHeader) error) error {
	return nil
}

func (s *chanSubscriber) NextHeader(ctx context.Context) (*header.ExtendedHeader, error) {
	select {
	case h := <-s.ch:
		return h, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *chanSubscriber) Cancel() {}

// TestDASer_RestartCaughtUpReportsDone shows that a DASer restarted from a checkpoint with
// nothing left to sample never reports catch-up (WaitCatchUp blocks, CatchUpDone=false,
// IsRunning=false) until some new header happens to arrive.
func TestDASer_RestartCaughtUpReportsDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	suite := headertest.NewTestSuite(t, headertest.WithBlockTime(time.Nanosecond))
	store := headertest.NewCustomStore(t, suite, 10)

	ctrl := gomock.NewController(t)
	avail := mocks.NewMockAvailability(ctrl)
	avail.EXPECT().SharesAvailable(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())

	daser, err := NewDASer(avail, &chanSubscriber{ch: make(chan *header.ExtendedHeader)}, store, ds)
	require.NoError(t, err)
	require.NoError(t, daser.Start(ctx))
	require.NoError(t, waitHeight(ctx, daser, 10))
	require.NoError(t, daser.Stop(ctx))

	// restart; the chain has not moved (e.g. halted for an upgrade) so there is nothing to sample
	daser, err = NewDASer(avail, &chanSubscriber{ch: make(chan *header.ExtendedHeader)}, store, ds)
	require.NoError(t, err)
	require.NoError(t, daser.Start(ctx))
	t.Cleanup(func() { _ = daser.Stop(context.Background()) })

	waitCtx, waitCancel := context.WithTimeout(ctx, 2*time.Second)
	defer waitCancel()
	err = daser.WaitCatchUp(waitCtx)
	stats, statsErr := daser.SamplingStats(ctx)
	require.NoError(t, statsErr)
	t.Logf("after restart: SampledChainHead=%d NetworkHead=%d CatchUpDone=%v IsRunning=%v WaitCatchUp err=%v",
		stats.SampledChainHead, stats.NetworkHead, stats.CatchUpDone, stats.IsRunning, err)
	require.NoError(t, err, "everything up to the network head is sampled, WaitCatchUp should return")
	require.True(t, stats.CatchUpDone)
}
