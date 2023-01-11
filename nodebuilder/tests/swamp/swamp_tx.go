package swamp

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client/flags"
)

// FillBlocks produces the given amount of contiguous blocks with customizable size.
// The returned channel reports when the process is finished.
func (s *Swamp) FillBlocks(ctx context.Context, bsize, blocks int) chan error {
	errCh := make(chan error)
	fmt.Println(s.accounts)
	go func() {
		// TODO: FillBlock must respect the context
		var err error
		for i := 0; i < blocks; i++ {
			_, err = s.ClientContext.FillBlock(bsize, s.accounts, flags.BroadcastBlock)
			if err != nil {
				err = fmt.Errorf("%v: failed on block %v", err, i)
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
