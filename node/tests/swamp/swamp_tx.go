package swamp

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/celestiaorg/celestia-node/ipld"
)

// SubmitData submits given data in the block.
// TODO(@Wondertan): This must be a real PFD using celestia-app, once we able to run App
//  in the Swamp.
func (s *Swamp) SubmitData(ctx context.Context, data []byte) error {
	result, err := s.CoreClient.BroadcastTxSync(ctx, append([]byte("key="), data...))
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("invalid status code: %d", result.Code)
	}

	return nil
}

func (s *Swamp) FillBlocks(ctx context.Context, bsize, blocks int) error {
	btime := s.comps.CoreCfg.Consensus.CreateEmptyBlocksInterval
	timer := time.NewTimer(btime)
	defer timer.Stop()

	data := make([]byte, bsize*ipld.ShareSize)
	for range make([]int, blocks) {
		rand.Read(data) //nolint:gosec
		if err := s.SubmitData(ctx, data); err != nil {
			return err
		}
		timer.Reset(btime)
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
