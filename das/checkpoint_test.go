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
				From: 1,
				To:   2,
			},
			{
				From: 5,
				To:   10,
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
