package swamp

import (
	"context"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	"github.com/celestiaorg/celestia-app/v7/test/util/testnode"
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
			txResp, err := cctx.FillBlock(bsize, account, flags.BroadcastSync)
			if err != nil {
				break
			}

			// ensure each tx is included before moving on to avoid account sequence mismatch errors in tests.
			resp, err := waitForTxResponse(cctx, txResp.TxHash, time.Second*10)
			if err != nil {
				break
			}
			heightCh <- uint64(resp.Height)
		}

		select {
		case errCh <- err:
		case <-ctx.Done():
		}
	}()
	return heightCh, errCh
}

// waitForTxResponse polls for a tx hash, returns an error if the query is not successful within the given timeout.
func waitForTxResponse(cctx testnode.Context, txHash string, timeout time.Duration) (*sdk.TxResponse, error) {
	ctx, cancel := context.WithTimeout(cctx.GoContext(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			resp, err := authtx.QueryTx(cctx.Context, txHash)
			if err == nil {
				return resp, nil
			}
		}
	}
}
