//go:build !race

package state

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v2/app"
	appconsts "github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	genesis "github.com/celestiaorg/celestia-app/v2/test/util/genesis"
	"github.com/celestiaorg/celestia-app/v2/test/util/testnode"
	apptypes "github.com/celestiaorg/celestia-app/v2/x/blob/types"
	squareblob "github.com/celestiaorg/go-square/blob"

	"github.com/celestiaorg/celestia-node/share"
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
	blobbyTheBlob, err := apptypes.NewBlob(ns.ToAppNamespace(), []byte("data"), 0)
	require.NoError(t, err)

	testcases := []struct {
		name     string
		blobs    []*squareblob.Blob
		gasPrice float64
		gasLim   uint64
		expErr   error
	}{
		{
			name:     "empty blobs",
			blobs:    []*squareblob.Blob{},
			gasPrice: DefaultGasPrice,
			gasLim:   0,
			expErr:   errors.New("state: no blobs provided"),
		},
		{
			name:     "good blob with user provided gas and fees",
			blobs:    []*squareblob.Blob{blobbyTheBlob},
			gasPrice: 0.005,
			gasLim:   apptypes.DefaultEstimateGas([]uint32{uint32(len(blobbyTheBlob.GetData()))}),
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

	minGas, err := ca.queryMinimumGasPrice(ctx)
	require.NoError(t, err)
	require.Equal(t, appconsts.DefaultMinGasPrice, minGas)

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
			name:     "transfer with options",
			gasPrice: 0.005,
			gasLim:   0,
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

	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0].Name, nil, "127.0.0.1", extractPort(grpcAddr))
	require.NoError(t, err)
	return ca, getNames(accounts)
}

func getNames(accounts []genesis.KeyringAccount) (names []string) {
	for _, account := range accounts {
		names = append(names, account.Name)
	}
	return names
}
