package swamp

import (
	"context"

	"github.com/celestiaorg/celestia-app/testutil/testnode"
)

// FillBlocks produces the given amount of contiguous blocks with customizable size.
// The returned channel reports when the process is finished.
func (s *Swamp) FillBlocks(ctx context.Context, bsize, blocks int) chan error {
	errCh := make(chan error)
	go func() {
		// TODO: FillBlock must respect the context
		_, err := testnode.FillBlock(s.ClientContext, bsize, s.accounts[:blocks+1])
		select {
		case errCh <- err:
		case <-ctx.Done():
		}
	}()
	return errCh
}
