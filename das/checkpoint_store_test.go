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
	ds := wrapCheckpointStore(sync.MutexWrap(datastore.NewMapDatastore()))
	checkpoint := int64(5)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer t.Cleanup(cancel)
	err := storeCheckpoint(ctx, ds, checkpoint)
	require.NoError(t, err)
	got, err := loadCheckpoint(ctx, ds)
	require.NoError(t, err)

	assert.Equal(t, checkpoint, got)
}
