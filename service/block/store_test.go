package block

import (
	"context"
	"testing"

	format "github.com/ipfs/go-ipld-format"
	md "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/ipld"

	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/service/header"
)

func TestService_BlockStore(t *testing.T) {
	// create mock block service (fetcher is not necessary here)
	mockStore := md.Mock()
	serv := NewBlockService(mockStore)

	block := generateRawAndExtendedBlock(t, serv.store)

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

func generateRawAndExtendedBlock(t *testing.T, store format.DAGService) *Block {
	mockCore := core.MockEmbeddedClient()
	fetcher := core.NewBlockFetcher(mockCore)
	sub, err := fetcher.SubscribeNewBlockEvent(context.Background())
	require.NoError(t, err)
	// generate 1 raw block
	<-sub
	// get that block
	rawBlock, err := fetcher.GetBlock(context.Background(), nil)
	require.NoError(t, err)
	// extend block
	shares, _ := rawBlock.ComputeShares()
	extended, err := ipld.PutData(context.Background(), shares.RawShares(), store)
	require.NoError(t, err)
	// generate dah
	dah := da.NewDataAvailabilityHeader(extended)

	return &Block{
		header: &header.ExtendedHeader{
			DAH: &dah,
		},
		data: extended,
	}
}
