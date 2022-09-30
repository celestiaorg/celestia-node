package swamp

import (
	"context"
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-app/testutil/testnode"
)

// SubmitData submits given data in the block.
// TODO(@Wondertan): This must be a real PFD using celestia-app, once we can run App in the Swamp.
func (s *Swamp) SubmitData(ctx context.Context, data []byte) error {
	result, err := s.ClientContext.Client.BroadcastTxSync(ctx, append([]byte("key="), data...))
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

	_, err := testnode.FillBlock(s.ClientContext, bsize, s.accounts)
	return err
}
