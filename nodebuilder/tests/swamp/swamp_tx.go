package swamp

import (
	"context"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	"github.com/celestiaorg/celestia-app/v4/test/util/testnode"
)

// FillBlocks produces the given amount of contiguous blocks with customizable size.
// The returned channel reports when the process is finished.
func FillBlocks(ctx context.Context, cctx testnode.Context, account string, bsize, blocks int) chan error {
	errCh := make(chan error)
	go func() {
		// TODO: FillBlock must respect the context
		// fill blocks is not working correctly without sleep rn.
		time.Sleep(time.Millisecond * 50)
		var err error
		for i := 0; i < blocks; i++ {
			var resp *sdk.TxResponse
			resp, err = cctx.FillBlock(bsize, account, flags.BroadcastSync)
			if err != nil {
				break
			}

			// ensure each tx is included before moving on to avoid account sequence mismatch errors in tests.
			err = waitForTxResponse(cctx, resp.TxHash, time.Second*1)
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

// waitForTxResponse polls for a tx hash, returns an error if the query is not successful within the given timeout.
func waitForTxResponse(cctx testnode.Context, txHash string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(cctx.GoContext(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_, err := authtx.QueryTx(cctx.Context, txHash)
			if err != nil {
				continue
			}
			return nil
		}
	}
}
