package swamp

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/ipld"
)

// SubmitData submits given data in the block.
// TODO(@Wondertan): This must be a real PFD using celestia-app, once we able to run App
//  in the Swamp.
func (s *Swamp) SubmitData(ctx context.Context, t *testing.T, data []byte) {
	result, err := s.CoreClient.BroadcastTxSync(ctx, append([]byte("key="), data...))
	require.NoError(t, err)
	require.Zero(t, result.Code)
}

func (s *Swamp) FillBlocks(ctx context.Context, t *testing.T, bsize, blocks int) {
	btime := s.comps.CoreCfg.Consensus.CreateEmptyBlocksInterval
	timer := time.NewTimer(btime)
	defer timer.Stop()

	data := make([]byte, bsize*ipld.ShareSize)
	for range make([]int, blocks) {
		rand.Read(data) //nolint:gosec
		s.SubmitData(ctx, t, data)

		timer.Reset(btime)
		select {
		case <-timer.C:
		case <-ctx.Done():
			return
		}
	}
}
