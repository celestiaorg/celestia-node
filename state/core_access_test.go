//go:build !race

package state

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/test/util/testnode"
	blobtypes "github.com/celestiaorg/celestia-app/x/blob/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share"
)

func TestSubmitPayForBlob(t *testing.T) {
	accounts := []string{"jimy", "rob"}
	tmCfg := testnode.DefaultTendermintConfig()
	tmCfg.Consensus.TimeoutCommit = time.Millisecond * 1
	appConf := testnode.DefaultAppConfig()
	appConf.API.Enable = true
	appConf.MinGasPrices = fmt.Sprintf("0.002%s", app.BondDenom)

	config := testnode.DefaultConfig().WithTendermintConfig(tmCfg).WithAppConfig(appConf).WithAccounts(accounts)
	cctx, _, grpcAddr := testnode.NewNetwork(t, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0], nil, "127.0.0.1", extractPort(grpcAddr))
	require.NoError(t, err)
	// start the accessor
	err = ca.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ca.Stop(ctx)
	})

	ns, err := share.NewBlobNamespaceV0([]byte("namespace"))
	require.NoError(t, err)
	blobbyTheBlob, err := blob.NewBlobV0(ns, []byte("data"))
	require.NoError(t, err)

	minGas, err := ca.queryMinimumGasPrice(ctx)
	require.NoError(t, err)
	require.Equal(t, appconsts.DefaultMinGasPrice, minGas)

	testcases := []struct {
		name            string
		blobs           []*blob.Blob
		fee             math.Int
		gasLim          uint64
		expErr          error
		asyncSubmission bool
	}{
		{
			name:            "empty blobs",
			blobs:           []*blob.Blob{},
			fee:             sdktypes.ZeroInt(),
			gasLim:          0,
			expErr:          errors.New("state: no blobs provided"),
			asyncSubmission: false,
		},
		{
			name:            "good blob with user provided gas and fees",
			blobs:           []*blob.Blob{blobbyTheBlob},
			fee:             sdktypes.NewInt(10_000), // roughly 0.12 utia per gas (should be good)
			gasLim:          blobtypes.DefaultEstimateGas([]uint32{uint32(len(blobbyTheBlob.Data))}),
			expErr:          nil,
			asyncSubmission: true,
		},
		// TODO: add more test cases. The problem right now is that the celestia-app doesn't
		// correctly construct the node (doesn't pass the min gas price) hence the price on
		// everything is zero and we can't actually test the correct behavior
	}

	// sync blob submission
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ca.SubmitPayForBlob(ctx, tc.fee, tc.gasLim, tc.blobs)
			require.Equal(t, tc.expErr, err)
			if err == nil {
				require.EqualValues(t, 0, resp.Code)
			}
		})
	}

	// async blob submission
	hashCh := make(chan string)
	for _, tc := range testcases {
		if tc.asyncSubmission {
			t.Run(tc.name, func(t *testing.T) {
				go func() {
					resp, err := ca.SubmitPayForBlob(ctx, tc.fee, tc.gasLim, tc.blobs)
					if err == nil {
						hashCh <- resp.TxHash
					}
				}()

				ctx, cancel := context.WithTimeout(ctx, time.Second*3)
				resp, err := ca.signer.ConfirmTx(ctx, <-hashCh)
				cancel()
				require.NoError(t, err)
				require.EqualValues(t, 0, resp.Code)
			})
		}
	}
}

func extractPort(addr string) string {
	splitStr := strings.Split(addr, ":")
	return splitStr[len(splitStr)-1]
}
