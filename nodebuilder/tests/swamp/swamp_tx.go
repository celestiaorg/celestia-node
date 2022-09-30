package swamp

import (
	"context"
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-app/testutil/testnode"
	"github.com/celestiaorg/celestia-app/x/payment"
	paytypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/tendermint/tendermint/pkg/consts"
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

func (s *Swamp) FillBlocks(ctx context.Context, bsize, blocks int) error {
	btime := s.comps.CoreCfg.Consensus.CreateEmptyBlocksInterval
	timer := time.NewTimer(btime)
	defer timer.Stop()
	for i := 0; i < blocks; i++ {
		_, err := testnode.FillBlock(s.ClientContext, bsize, s.accounts)
		if err != nil {
			return err
		}
		err = testnode.WaitForNextBlock(s.ClientContext)
		if err != nil {
			return err
		}
	}
	return nil
}
