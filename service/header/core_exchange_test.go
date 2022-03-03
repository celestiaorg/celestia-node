package header

import (
	"bytes"
	"context"
	"testing"
	"time"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
)

func TestCoreExchange_RequestHeaders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	fetcher, err := createCoreFetcher(ctx, t)
	require.NoError(t, err)
	store := mdutils.Mock()

	// generate 10 blocks
	generateBlocks(t, fetcher)

	ce := NewCoreExchange(fetcher, store)
	headers, err := ce.RequestHeaders(ctx, 1, 10)
	require.NoError(t, err)

	assert.Equal(t, 10, len(headers))
}

func Test_hashMatch(t *testing.T) {
	expected := []byte("AE0F153556A4FA5C0B7C3BFE0BAF0EC780C031933B281A8D759BB34C1DA31C56")
	mismatch := []byte("57A0D7FE69FE88B3D277C824B3ACB9B60E5E65837A802485DE5CBB278C43576A")

	assert.False(t, bytes.Equal(expected, mismatch))
}

func createCoreFetcher(ctx context.Context, t *testing.T) (*core.BlockFetcher, error) {
	mock := core.EphemeralMockEmbeddedClient(t)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if mock.IsRunning() {
				return core.NewBlockFetcher(mock), nil
			}
		}
	}
}

func generateBlocks(t *testing.T, fetcher *core.BlockFetcher) {
	sub, err := fetcher.SubscribeNewBlockEvent(context.Background())
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		<-sub
	}
}
