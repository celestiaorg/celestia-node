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
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/test/util/testnode"
	blobtypes "github.com/celestiaorg/celestia-app/x/blob/types"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/state/options"
)

func TestSubmitPayForBlob(t *testing.T) {
	ctx := context.Background()
	ca, accounts := buildAccessor(t)
	// start the accessor
	err := ca.Start(ctx)
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
		name   string
		blobs  []*blob.Blob
		fee    math.Int
		gasLim uint64
		expErr error
	}{
		{
			name:   "empty blobs",
			blobs:  []*blob.Blob{},
			fee:    sdktypes.ZeroInt(),
			gasLim: 0,
			expErr: errors.New("state: no blobs provided"),
		},
		{
			name:   "good blob with user provided gas and fees",
			blobs:  []*blob.Blob{blobbyTheBlob},
			fee:    sdktypes.NewInt(10_000), // roughly 0.12 utia per gas (should be good)
			gasLim: blobtypes.DefaultEstimateGas([]uint32{uint32(len(blobbyTheBlob.Data))}),
			expErr: nil,
		},
		// TODO: add more test cases. The problem right now is that the celestia-app doesn't
		// correctly construct the node (doesn't pass the min gas price) hence the price on
		// everything is zero and we can't actually test the correct behavior
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ca.SubmitPayForBlob(ctx, tc.blobs, options.DefaultTxOptions())
			require.Equal(t, tc.expErr, err)
			if err == nil {
				require.EqualValues(t, 0, resp.Code)
			}
		})
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			opts := options.DefaultTxOptions()
			opts.Gas = tc.gasLim
			opts.SetFeeAmount(tc.fee.Int64())
			opts.AccountKey = accounts[2]
			resp, err := ca.SubmitPayForBlob(ctx, tc.blobs, opts)
			require.Equal(t, tc.expErr, err)
			if err == nil {
				require.EqualValues(t, 0, resp.Code)
			}
		})
	}
}

func TestTransfer(t *testing.T) {
	ctx := context.Background()
	ca, accounts := buildAccessor(t)
	// start the accessor
	err := ca.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ca.Stop(ctx)
	})

	minGas, err := ca.queryMinimumGasPrice(ctx)
	require.NoError(t, err)
	require.Equal(t, appconsts.DefaultMinGasPrice, minGas)

	testcases := []struct {
		name    string
		fee     int64
		gasLim  uint64
		account string
		expErr  error
	}{
		{
			name:    "transfer without options",
			fee:     -1,
			gasLim:  0,
			account: "",
			expErr:  nil,
		},
		{
			name:    "transfer with options",
			fee:     2_000,
			gasLim:  0,
			account: accounts[2],
			expErr:  nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			opts := options.DefaultTxOptions()
			opts.SetFeeAmount(tc.fee)
			opts.Gas = tc.gasLim
			if tc.account != "" {
				opts.AccountKey = tc.account
			}

			key, err := ca.keyring.Key(accounts[1])
			require.NoError(t, err)
			addr, err := key.GetAddress()
			require.NoError(t, err)

			resp, err := ca.Transfer(ctx, addr, sdktypes.NewInt(10_000), opts)
			require.Equal(t, tc.expErr, err)
			if err == nil {
				require.EqualValues(t, 0, resp.Code)
			}
		})
	}
}

func TestDelegate(t *testing.T) {
	ctx := context.Background()
	ca, accounts := buildAccessor(t)
	// start the accessor
	err := ca.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ca.Stop(ctx)
	})

	minGas, err := ca.queryMinimumGasPrice(ctx)
	require.NoError(t, err)
	require.Equal(t, appconsts.DefaultMinGasPrice, minGas)

	valRec, err := ca.keyring.Key("validator")
	require.NoError(t, err)
	valAddr, err := valRec.GetAddress()
	require.NoError(t, err)

	testcases := []struct {
		name    string
		fee     int64
		gasLim  uint64
		account string
	}{
		{
			name:    "delegate/undelegate without options",
			fee:     -1,
			gasLim:  0,
			account: "",
		},
		{
			name:    "delegate/undelegate with options",
			fee:     2_000,
			gasLim:  0,
			account: accounts[2],
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			opts := options.DefaultTxOptions()
			opts.SetFeeAmount(tc.fee)
			opts.Gas = tc.gasLim
			if tc.account != "" {
				opts.AccountKey = tc.account
			}

			resp, err := ca.Delegate(ctx, ValAddress(valAddr), sdktypes.NewInt(100_000), opts)
			require.NoError(t, err)
			require.EqualValues(t, 0, resp.Code)

			// reset for empty case
			opts = options.DefaultTxOptions()
			opts.SetFeeAmount(tc.fee)
			opts.Gas = tc.gasLim
			if tc.account != "" {
				opts.AccountKey = tc.account
			}
			resp, err = ca.Undelegate(ctx, ValAddress(valAddr), sdktypes.NewInt(100_000), opts)
			require.NoError(t, err)
			require.EqualValues(t, 0, resp.Code)
		})
	}
}

func extractPort(addr string) string {
	splitStr := strings.Split(addr, ":")
	return splitStr[len(splitStr)-1]
}

func buildAccessor(t *testing.T) (*CoreAccessor, []string) {
	t.Helper()
	accounts := []string{"jimmy", "carl", "sheen", "cindy"}
	tmCfg := testnode.DefaultTendermintConfig()
	tmCfg.Consensus.TimeoutCommit = time.Millisecond * 1

	appConf := testnode.DefaultAppConfig()
	appConf.API.Enable = true
	appConf.MinGasPrices = fmt.Sprintf("0.002%s", app.BondDenom)

	config := testnode.DefaultConfig().WithTendermintConfig(tmCfg).WithAppConfig(appConf).WithAccounts(accounts)
	cctx, _, grpcAddr := testnode.NewNetwork(t, config)

	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0], nil, "127.0.0.1", extractPort(grpcAddr))
	require.NoError(t, err)
	return ca, accounts
}
