package eds

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/ipld"
)

func TestBlockGetter_GetBlocks(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		cids := randCIDs(32)
		// sort cids in asc order
		sort.Slice(cids, func(i, j int) bool {
			return cids[i].String() < cids[j].String()
		})

		bg := &BlockGetter{store: rbsMock{}}
		blocksCh := bg.GetBlocks(context.Background(), cids)

		// collect blocks from channel
		blocks := make([]blocks.Block, 0, len(cids))
		for block := range blocksCh {
			blocks = append(blocks, block)
		}

		// sort blocks in cid asc order
		sort.Slice(blocks, func(i, j int) bool {
			return blocks[i].Cid().String() < blocks[j].Cid().String()
		})

		// validate results
		require.Equal(t, len(cids), len(blocks))
		for i, block := range blocks {
			require.Equal(t, cids[i].String(), block.Cid().String())
		}
	})
	t.Run("retrieval error", func(t *testing.T) {
		cids := randCIDs(32)

		// split cids into failed and succeeded
		failedLen := rand.Intn(len(cids)-1) + 1
		failed := make(map[cid.Cid]struct{}, failedLen)
		succeeded := make([]cid.Cid, 0, len(cids)-failedLen)
		for i, cid := range cids {
			if i < failedLen {
				failed[cid] = struct{}{}
				continue
			}
			succeeded = append(succeeded, cid)
		}

		// sort succeeded cids in asc order
		sort.Slice(succeeded, func(i, j int) bool {
			return succeeded[i].String() < succeeded[j].String()
		})

		bg := &BlockGetter{store: rbsMock{failed: failed}}
		blocksCh := bg.GetBlocks(context.Background(), cids)

		// collect blocks from channel
		blocks := make([]blocks.Block, 0, len(cids))
		for block := range blocksCh {
			blocks = append(blocks, block)
		}

		// sort blocks in cid asc order
		sort.Slice(blocks, func(i, j int) bool {
			return blocks[i].Cid().String() < blocks[j].Cid().String()
		})

		// validate results
		require.Equal(t, len(succeeded), len(blocks))
		for i, block := range blocks {
			require.Equal(t, succeeded[i].String(), block.Cid().String())
		}
	})
	t.Run("retrieval timeout", func(t *testing.T) {
		cids := randCIDs(128)

		bg := &BlockGetter{
			store: rbsMock{},
		}

		// cancel the context before any blocks are collected
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		blocksCh := bg.GetBlocks(ctx, cids)

		// pretend nobody is reading from blocksCh after context is canceled
		time.Sleep(50 * time.Millisecond)

		// blocksCh should be closed indicating GetBlocks exited
		select {
		case _, ok := <-blocksCh:
			require.False(t, ok)
		default:
			t.Error("channel is not closed on canceled context")
		}
	})
}

// rbsMock is a dagstore.ReadBlockstore mock
type rbsMock struct {
	failed map[cid.Cid]struct{}
}

func (r rbsMock) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	panic("implement me")
}

func (r rbsMock) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	// return error for failed items
	if _, ok := r.failed[cid]; ok {
		return nil, errors.New("not found")
	}

	return blocks.NewBlockWithCid(nil, cid)
}

func (r rbsMock) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	panic("implement me")
}

func (r rbsMock) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	panic("implement me")
}

func (r rbsMock) HashOnRead(enabled bool) {
	panic("implement me")
}

func randCID() cid.Cid {
	hash := make([]byte, ipld.NmtHashSize)
	rand.Read(hash)
	return ipld.MustCidFromNamespacedSha256(hash)
}

func randCIDs(n int) []cid.Cid {
	cids := make([]cid.Cid, n)
	for i := range cids {
		cids[i] = randCID()
	}
	return cids
}
