package swamp

import (
	"context"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/celestiaorg/celestia-app/testutil/testnode"
)

// FillBlocks produces the given amount of contiguous blocks with customizable size.
// The returned channel reports when the process is finished.
func FillBlocks(ctx context.Context, cctx testnode.Context, accounts []string, bsize, blocks int) chan error {
	errCh := make(chan error)
	go func() {
		// TODO: FillBlock must respect the context
		// fill blocks is not working correctly without sleep rn.
		time.Sleep(time.Millisecond * 50)
		var err error
		for i := 0; i < blocks; i++ {
			_, err = cctx.FillBlock(bsize, accounts, flags.BroadcastBlock)
			if err != nil {
				break
			}
		}

		select {
		case errCh <- err:
		case <-ctx.Done():
		}
	}()
	return errCh
}
