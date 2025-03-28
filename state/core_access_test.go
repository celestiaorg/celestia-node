//go:build !race

package state

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v3/app"
	"github.com/celestiaorg/celestia-app/v3/test/util/genesis"
	"github.com/celestiaorg/celestia-app/v3/test/util/testnode"
	apptypes "github.com/celestiaorg/celestia-app/v3/x/blob/types"
	libshare "github.com/celestiaorg/go-square/v2/share"
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
	// explicitly reset client to nil to ensure
	// that retry mechanism works.
	ca.client = nil

	ns, err := libshare.NewV0Namespace([]byte("namespace"))
	require.NoError(t, err)
	require.False(t, ns.IsReserved())

	require.NoError(t, err)
	blobbyTheBlob, err := libshare.NewV0Blob(ns, []byte("data"))
	require.NoError(t, err)

	testcases := []struct {
		name     string
		blobs    []*libshare.Blob
		gasPrice float64
		gasLim   uint64
		expErr   error
	}{
		{
			name:     "empty blobs",
			blobs:    []*libshare.Blob{},
			gasPrice: DefaultGasPrice,
			gasLim:   0,
			expErr:   errors.New("state: no blobs provided"),
		},
		{
			name:     "good blob with user provided gas and fees",
			blobs:    []*libshare.Blob{blobbyTheBlob},
			gasPrice: 0.005,
			gasLim:   apptypes.DefaultEstimateGas([]uint32{uint32(blobbyTheBlob.DataLen())}),
			expErr:   nil,
		},
		// TODO: add more test cases. The problem right now is that the celestia-app doesn't
		// correctly construct the node (doesn't pass the min gas price) hence the price on
		// everything is zero and we can't actually test the correct behavior
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ca.SubmitPayForBlob(ctx, tc.blobs, NewTxConfig())
			require.Equal(t, tc.expErr, err)
			if err == nil {
				require.EqualValues(t, 0, resp.Code)
			}
		})
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			opts := NewTxConfig(
				WithGas(tc.gasLim),
				WithGasPrice(tc.gasPrice),
				WithKeyName(accounts[2]),
			)
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

	testcases := []struct {
		name     string
		gasPrice float64
		gasLim   uint64
		account  string
		expErr   error
	}{
		{
			name:     "transfer without options",
			gasPrice: DefaultGasPrice,
			gasLim:   0,
			account:  "",
			expErr:   nil,
		},
		{
			name:     "transfer with gasPrice set",
			gasPrice: 0.005,
			gasLim:   0,
			account:  accounts[2],
			expErr:   nil,
		},
		{
			name:     "transfer with gas set",
			gasPrice: DefaultGasPrice,
			gasLim:   84617,
			account:  accounts[2],
			expErr:   nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			opts := NewTxConfig(
				WithGas(tc.gasLim),
				WithGasPrice(tc.gasPrice),
				WithKeyName(accounts[2]),
			)
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

func TestChainIDMismatch(t *testing.T) {
	ctx := context.Background()
	ca, _ := buildAccessor(t)
	ca.network = "mismatch"
	// start the accessor
	err := ca.Start(ctx)
	assert.ErrorContains(t, err, "wrong network")
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

	valRec, err := ca.keyring.Key("validator")
	require.NoError(t, err)
	valAddr, err := valRec.GetAddress()
	require.NoError(t, err)

	testcases := []struct {
		name     string
		gasPrice float64
		gasLim   uint64
		account  string
	}{
		{
			name:     "delegate/undelegate without options",
			gasPrice: DefaultGasPrice,
			gasLim:   0,
			account:  "",
		},
		{
			name:     "delegate/undelegate with options",
			gasPrice: 0.005,
			gasLim:   0,
			account:  accounts[2],
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			opts := NewTxConfig(
				WithGas(tc.gasLim),
				WithGasPrice(tc.gasPrice),
				WithKeyName(accounts[2]),
			)
			resp, err := ca.Delegate(ctx, ValAddress(valAddr), sdktypes.NewInt(100_000), opts)
			require.NoError(t, err)
			require.EqualValues(t, 0, resp.Code)

			opts = NewTxConfig(
				WithGas(tc.gasLim),
				WithGasPrice(tc.gasPrice),
				WithKeyName(accounts[2]),
			)

			resp, err = ca.Undelegate(ctx, ValAddress(valAddr), sdktypes.NewInt(100_000), opts)
			require.NoError(t, err)
			require.EqualValues(t, 0, resp.Code)
		})
	}
}

func TestEstimatorService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mes := setupEstimatorService(t)

	ca, _ := buildAccessor(t, WithEstimatorService(mes.addr))
	// start the accessor
	err := ca.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ca.Stop(ctx)
	})

	// should estimate gas price using estimator service
	t.Run("tx with default gas price", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02

		txConfig := NewTxConfig()
		gasPrice, err := ca.estimateGasPrice(ctx, txConfig)
		require.NoError(t, err)
		assert.Equal(t, mes.gasPriceToReturn, gasPrice)
	})
	// should return configured gas price instead of estimated one
	t.Run("tx with manually configured gas price", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02

		txConfig := NewTxConfig(WithGasPrice(0.005))
		gasPrice, err := ca.estimateGasPrice(ctx, txConfig)
		require.NoError(t, err)
		assert.Equal(t, 0.005, gasPrice)
	})
	// should error out with ErrGasPriceExceedsLimit
	t.Run("estimator exceeds configured max gas price", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02

		txConfig := NewTxConfig(WithMaxGasPrice(0.0001))
		_, err := ca.estimateGasPrice(ctx, txConfig)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrGasPriceExceedsLimit)
	})
	// should use estimated gas price as it is lower than max gas price
	t.Run("estimated gas price is lower than max gas price, succeeds", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02

		txConfig := NewTxConfig(WithMaxGasPrice(0.1))
		gasPrice, err := ca.estimateGasPrice(ctx, txConfig)
		require.NoError(t, err)
		assert.Equal(t, mes.gasPriceToReturn, gasPrice)
	})
	t.Run("estimate gas price AND usage", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02
		mes.gasUsageToReturn = 10000

		// dummy tx, doesn't matter what's in it
		coins := sdktypes.NewCoins(sdktypes.NewCoin(app.BondDenom, math.NewInt(100)))
		msg := banktypes.NewMsgSend(ca.defaultSignerAddress, ca.defaultSignerAddress, coins)

		testcases := []struct {
			name        string
			txconf      *TxConfig
			expGasPrice float64
			expGasUsage uint64
			errExpected bool
			expectedErr error
		}{
			{
				// should return both the configured gas price and limit
				name:        "configured gas price and limit",
				txconf:      NewTxConfig(WithGasPrice(0.005), WithGas(2000)),
				expGasPrice: 0.005,
				expGasUsage: 2000,
			},
			{
				// should return the configured gas price and estimated gas usage
				name:        "configured gas price only",
				txconf:      NewTxConfig(WithGasPrice(0.005)),
				expGasPrice: 0.005,
				expGasUsage: 10000 * 1.1, // gas multiplier
			},
			{
				// should return estimated gas price and gas usage
				name:        "estimate gas price and usage",
				txconf:      NewTxConfig(),
				expGasPrice: 0.02,
				expGasUsage: 10000 * 1.1, // gas multiplier
			},
			{
				// should return error as gas price exceeds limit
				name:        "estimate gas price exceeds max gas price",
				txconf:      NewTxConfig(WithMaxGasPrice(0.0001)),
				errExpected: true,
				expectedErr: ErrGasPriceExceedsLimit,
			},
			{
				// should return error as gas price exceeds limit
				name:        "estimate gas price, use configured gas",
				txconf:      NewTxConfig(WithGas(100)),
				expGasPrice: 0.02,
				expGasUsage: 100,
			},
		}

		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				gasPrice, gasUsage, err := ca.estimateGasPriceAndUsage(ctx, tc.txconf, msg)
				if tc.errExpected {
					assert.Error(t, err)
					assert.ErrorIs(t, err, tc.expectedErr)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tc.expGasPrice, gasPrice)
				assert.Equal(t, tc.expGasUsage, gasUsage)
			})
		}
	})
}

func buildAccessor(t *testing.T, opts ...Option) (*CoreAccessor, []string) {
	chainID := "private"

	t.Helper()
	accounts := []genesis.KeyringAccount{
		{
			Name:          "jimmy",
			InitialTokens: 100_000_000,
		},
		{
			Name:          "carl",
			InitialTokens: 100_000_000,
		},
		{
			Name:          "sheen",
			InitialTokens: 100_000_000,
		},
		{
			Name:          "cindy",
			InitialTokens: 100_000_000,
		},
	}
	tmCfg := testnode.DefaultTendermintConfig()
	tmCfg.Consensus.TimeoutCommit = time.Millisecond * 1

	appConf := testnode.DefaultAppConfig()
	appConf.API.Enable = true

	appCreator := testnode.CustomAppCreator(fmt.Sprintf("0.002%s", app.BondDenom))

	g := genesis.NewDefaultGenesis().
		WithChainID(chainID).
		WithValidators(genesis.NewDefaultValidator(testnode.DefaultValidatorAccountName)).
		WithConsensusParams(testnode.DefaultConsensusParams()).WithKeyringAccounts(accounts...)

	config := testnode.DefaultConfig().
		WithChainID(chainID).
		WithTendermintConfig(tmCfg).
		WithAppConfig(appConf).
		WithGenesis(g).
		WithAppCreator(appCreator) // needed until https://github.com/celestiaorg/celestia-app/pull/3680 merges
	cctx, _, grpcAddr := testnode.NewNetwork(t, config)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0].Name, nil, conn, chainID, opts...)
	require.NoError(t, err)
	return ca, getNames(accounts)
}

func getNames(accounts []genesis.KeyringAccount) (names []string) {
	for _, account := range accounts {
		names = append(names, account.Name)
	}
	return names
}
