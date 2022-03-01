package das

import (
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckpointStore(t *testing.T) {
	ds := wrapCheckpointStore(sync.MutexWrap(datastore.NewMapDatastore()))
	checkpoint := int64(5)
	err := storeCheckpoint(ds, checkpoint)
	require.NoError(t, err)
	got, err := loadCheckpoint(ds)
	require.NoError(t, err)

	assert.Equal(t, checkpoint, got)
}
