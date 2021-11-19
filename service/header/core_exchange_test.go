package header

import (
	"context"
	"fmt"
	"testing"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
)

func TestCoreExchange_RequestHeaders(t *testing.T) {
	fetcher := createCoreFetcher()
	store := mdutils.Mock()

	// generate 10 blocks
	generateBlocks(t, fetcher)

	ce := NewCoreExchange(fetcher, store)
	headers, err := ce.RequestHeaders(context.Background(), 0, 10)
	require.NoError(t, err)

	fmt.Println(len(headers))
}

func createCoreFetcher() *core.BlockFetcher {
	mock := core.MockEmbeddedClient()
	return core.NewBlockFetcher(mock)
}

func generateBlocks(t *testing.T, fetcher *core.BlockFetcher) {
	sub, err := fetcher.SubscribeNewBlockEvent(context.Background())
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		<-sub
	}
}
