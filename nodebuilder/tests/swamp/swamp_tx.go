package swamp

import (
	"context"
	"fmt"

	"github.com/tendermint/tendermint/pkg/consts"

	"github.com/celestiaorg/celestia-app/testutil/testnode"
	"github.com/celestiaorg/celestia-app/x/payment"
	paytypes "github.com/celestiaorg/celestia-app/x/payment/types"
)

// SubmitData submits given data in the block.
// TODO(@Wondertan): This must be a real PFD using celestia-app, once we can run App in the Swamp.
func (s *Swamp) SubmitData(ctx context.Context, data []byte) error {
	signer := paytypes.NewKeyringSigner(s.ClientContext.Keyring, s.accounts[0], s.ClientContext.ChainID)
	resp, err := payment.SubmitPayForData(ctx, signer, s.ClientContext.GRPCClient, data[:consts.NamespaceSize], data, 10000000000)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("invalid status code: %d", resp.Code)
	}
	return nil
}

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
