package block

import (
	"context"
	"testing"

	md "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_BlockStore(t *testing.T) {
	// create mock block service (fetcher is not necessary here)
	mockStore := md.Mock()
	serv := NewBlockService(nil, mockStore)

	_, block := generateRawAndExtendedBlock(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := serv.StoreBlockData(ctx, block.Data())
	require.NoError(t, err)

	eds, err := serv.GetBlockData(ctx, block.Header().DAH)
	require.NoError(t, err)
	assert.Equal(t, block.data.Width(), eds.Width())
	assert.Equal(t, block.data.RowRoots(), eds.RowRoots())
	assert.Equal(t, block.data.ColRoots(), eds.ColRoots())
}
