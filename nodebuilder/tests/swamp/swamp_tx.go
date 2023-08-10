package swamp

import (
	"context"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/test/txsim"
	"github.com/celestiaorg/celestia-app/test/util/testnode"
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

func FillBlocksWithMultipleBlobs(t *testing.T, ctx context.Context, cfg *testnode.Config, sdkcontext testnode.Context) {
	err := txsim.Run(
		ctx,
		[]string{cfg.TmConfig.RPC.ListenAddress},
		[]string{cfg.AppConfig.GRPC.Address},
		sdkcontext.Keyring, 9001, 100*time.Millisecond,
		txsim.NewBlobSequence(txsim.NewRange(1e3, 1e4), txsim.NewRange(1, 1)).Clone(10)...,
	)
	require.Equal(t, context.DeadlineExceeded, err)
}
