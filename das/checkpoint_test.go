package das

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckpointStore(t *testing.T) {
	ds := newCheckpointStore(sync.MutexWrap(datastore.NewMapDatastore()))
	failed := make(map[uint64]int)
	failed[2] = 1
	failed[3] = 2
	cp := checkpoint{
		SampleFrom:  1,
		NetworkHead: 6,
		Failed:      failed,
		Workers: []workerCheckpoint{
			{
				From:    1,
				To:      2,
				JobType: retryJob,
			},
			{
				From:    5,
				To:      10,
				JobType: recentJob,
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer t.Cleanup(cancel)
	assert.NoError(t, ds.store(ctx, cp))
	got, err := ds.load(ctx)
	require.NoError(t, err)
	assert.Equal(t, cp, got)
}

// TestNewCheckpointKeepsInFlightRecentJob checks that a recent job still running when the
// checkpoint is taken is resumed after restart, as catchup has already moved past it.
func TestNewCheckpointKeepsInFlightRecentJob(t *testing.T) {
	stats := SamplingStats{
		CatchupHead: 11,
		Workers: []WorkerStats{
			{JobType: recentJob, Curr: 11, From: 11, To: 11},
			{JobType: recentJob, Curr: 15, From: 15, To: 15}, // covered by catchup from 12
		},
	}
	cp := newCheckpoint(stats)
	require.Equal(t, []workerCheckpoint{{From: 11, To: 11, JobType: catchupJob}}, cp.Workers)
}
