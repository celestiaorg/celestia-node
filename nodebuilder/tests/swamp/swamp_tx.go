package swamp

import (
	"context"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/celestiaorg/celestia-app/v3/test/util/testnode"
)

// FillBlocks produces the given amount of contiguous blocks with customizable size.
// The returned channels report the submitted heights and when the process is finished.
func FillBlocks(
	ctx context.Context,
	cctx testnode.Context,
	account string,
	bsize, blocks int,
) (
	<-chan uint64, <-chan error,
) {
	errCh := make(chan error, 1)
	heightCh := make(chan uint64, blocks)
	go func() {
		defer close(errCh)
		defer close(heightCh)
		// TODO: FillBlock must respect the context
		// fill blocks is not working correctly without sleep rn.
		time.Sleep(time.Millisecond * 50)
		var err error
		for range blocks {
			txResp, err := cctx.FillBlock(bsize, account, flags.BroadcastBlock)
			if err != nil {
				break
			}
			heightCh <- uint64(txResp.Height)
		}

		select {
		case errCh <- err:
		case <-ctx.Done():
		}
	}()
	return heightCh, errCh
}
