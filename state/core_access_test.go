//go:build !race

package state

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v7/test/util/testnode"
	apptypes "github.com/celestiaorg/celestia-app/v7/x/blob/types"
	libshare "github.com/celestiaorg/go-square/v3/share"
)

const (
	chainID = "private"
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
			blobs:    []*libshare.Blob{blobbyTheBlob, blobbyTheBlob},
			gasPrice: 0.005,
			gasLim: apptypes.DefaultEstimateGas(&apptypes.MsgPayForBlobs{
				BlobSizes:     []uint32{uint32(blobbyTheBlob.DataLen()), uint32(blobbyTheBlob.DataLen())},
				ShareVersions: []uint32{uint32(blobbyTheBlob.ShareVersion()), uint32(blobbyTheBlob.ShareVersion())},
			}),
			expErr: nil,
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

			resp, err := ca.Transfer(ctx, addr, math.NewInt(10_000), opts)
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
			resp, err := ca.Delegate(ctx, ValAddress(valAddr), math.NewInt(100_000), opts)
			require.NoError(t, err)
			require.EqualValues(t, 0, resp.Code)

			opts = NewTxConfig(
				WithGas(tc.gasLim),
				WithGasPrice(tc.gasPrice),
				WithKeyName(accounts[2]),
			)

			resp, err = ca.Undelegate(ctx, ValAddress(valAddr), math.NewInt(100_000), opts)
			require.NoError(t, err)
			require.EqualValues(t, 0, resp.Code)
		})
	}
}

func TestWithdrawDelegatorReward(t *testing.T) {
	ctx := context.Background()
	ca, _ := buildAccessor(t)
	err := ca.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ca.Stop(ctx)
	})

	valRec, err := ca.keyring.Key("validator")
	require.NoError(t, err)
	valAddr, err := valRec.GetAddress()
	require.NoError(t, err)

	// use default signer (accounts[0]) so QueryDelegationRewards matches
	opts := NewTxConfig()

	// delegate first so there is an active delegation
	resp, err := ca.Delegate(ctx, ValAddress(valAddr), math.NewInt(100_000), opts)
	require.NoError(t, err)
	require.EqualValues(t, 0, resp.Code)

	// query rewards — should succeed even if rewards are zero right after delegation
	rewardsResp, err := ca.QueryDelegationRewards(ctx, ValAddress(valAddr))
	require.NoError(t, err)
	require.NotNil(t, rewardsResp)

	// withdraw rewards — should succeed even with zero rewards
	opts = NewTxConfig()
	resp, err = ca.WithdrawDelegatorReward(ctx, ValAddress(valAddr), opts)
	require.NoError(t, err)
	require.EqualValues(t, 0, resp.Code)
}

func TestParallelPayForBlobSubmission(t *testing.T) {
	const (
		workerAccounts = 4
		blobCount      = 10
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	t.Helper()
	accounts := []string{
		"jimmy", "carl", "sheen", "cindy",
	}

	config := testnode.DefaultConfig().
		WithChainID(chainID).
		WithFundedAccounts(accounts...).
		WithDelayedPrecommitTimeout(time.Millisecond)

	cctx, _, grpcAddr := testnode.NewNetwork(t, config)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0], nil, conn, chainID, nil, WithTxWorkerAccounts(workerAccounts))
	require.NoError(t, err)
	err = ca.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ca.Stop(ctx)
	})

	blobs := make([][]*libshare.Blob, blobCount)
	for i := 0; i < blobCount; i++ {
		generated, err := libshare.GenerateV0Blobs([]int{8}, false)
		require.NoError(t, err)
		blobs[i] = generated
	}

	responses := make([]*TxResponse, blobCount)
	var g errgroup.Group

	for i := 0; i < blobCount; i++ {
		idx := i
		g.Go(func() error {
			resp, err := ca.SubmitPayForBlob(ctx, blobs[idx], NewTxConfig())
			if err != nil {
				return err
			}
			if resp == nil {
				return fmt.Errorf("nil response for blob %d", idx)
			}
			if resp.Code != 0 {
				return fmt.Errorf("unexpected code for blob %d: %d", idx, resp.Code)
			}
			responses[idx] = resp
			return nil
		})
	}

	require.NoError(t, g.Wait())

	hashes := make(map[string]struct{}, blobCount)
	for _, resp := range responses {
		require.NotNil(t, resp)
		hashes[resp.TxHash] = struct{}{}
	}
	require.Len(t, hashes, blobCount)

	for i := 1; i < workerAccounts; i++ {
		name := fmt.Sprintf("parallel-worker-%d", i)
		_, err := ca.keyring.Key(name)
		require.NoError(t, err, "expected worker account %s", name)
	}
}

// TestTxWorkerSetup ensures that the tx worker setup works properly
// despite having some pre-existing parallel worker accounts existing
// in the node's keyring, both funded and unfunded.
// Ref: https://github.com/celestiaorg/celestia-app/pull/6014
func TestTxWorkerSetup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	accounts := []string{
		// fund a parallel tx worker account so it exists in account state
		"jimmy", "carl", "sheen", "cindy", "parallel-worker-5",
	}

	config := testnode.DefaultConfig().
		WithChainID(chainID).
		WithFundedAccounts(accounts...).
		WithDelayedPrecommitTimeout(time.Millisecond)

	cctx, _, grpcAddr := testnode.NewNetwork(t, config)
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	// create parallel tx worker in keyring (but it is NOT YET FUNDED)
	path := hd.CreateHDPath(sdktypes.CoinType, 0, 0).String()
	_, _, err = cctx.Keyring.NewMnemonic("parallel-worker-2", keyring.English, path,
		keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	require.NoError(t, err)

	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0], nil, conn, chainID, nil, WithTxWorkerAccounts(8))
	require.NoError(t, err)
	err = ca.Start(ctx)
	require.NoError(t, err)
	// ensure tx client is set up properly even though some parallel worker accounts
	// exist in keyring already (unfunded) and some are funded
	err = ca.setupTxClient(ctx)
	require.NoError(t, err)
}

// TestSubmitFromDefaultAccountWithoutTxWorkers ensures users can submit transactions
// bypassing the queue from the default account
func TestSubmitFromDefaultAccountWithoutTxWorkers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	accounts := []string{
		"jimmy", "carl", "sheen", "cindy",
	}

	config := testnode.DefaultConfig().
		WithChainID(chainID).
		WithFundedAccounts(accounts...).
		WithDelayedPrecommitTimeout(time.Millisecond)

	cctx, _, grpcAddr := testnode.NewNetwork(t, config)
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	// configured without txworkers
	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0], localHeader{cctx.Client}, conn, chainID, nil)
	require.NoError(t, err)
	err = ca.Start(ctx)
	require.NoError(t, err)

	randBlob, err := libshare.GenerateV0Blobs([]int{8}, false)
	require.NoError(t, err)

	nonDefaultAcct, err := cctx.Keyring.Key(accounts[3])
	require.NoError(t, err)
	addr, err := nonDefaultAcct.GetAddress()
	require.NoError(t, err)

	// check bals of accounts before tx (TODO @renaynay: hack til we get signer in txresp)
	sdkAddress, err := sdktypes.AccAddressFromHexUnsafe(fmt.Sprintf("%X", addr.Bytes()))
	require.NoError(t, err)
	balNonDefault, err := ca.BalanceForAddress(ctx, Address{sdkAddress})
	require.NoError(t, err)
	balDefault, err := ca.Balance(ctx)
	require.NoError(t, err)

	// default tx config (submit from default acct)
	_, err = ca.SubmitPayForBlob(ctx, randBlob, NewTxConfig())
	require.NoError(t, err)

	// ensure balance has remained the same for non-default account
	updatedBalNonDefault, err := ca.BalanceForAddress(ctx, Address{sdkAddress})
	require.NoError(t, err)
	require.True(t, updatedBalNonDefault.Amount.Equal(balNonDefault.Amount))

	// ensure balance decreased for default account
	updatedBalDefault, err := ca.Balance(ctx)
	require.NoError(t, err)
	require.True(t, updatedBalDefault.Amount.LT(balDefault.Amount))

	// TODO @renaynay: once tx response contains signer, check signer here
}

// TestSubmitFromCustomAccount ensures users can submit transactions
// from a non-default account
func TestSubmitFromCustomAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	accounts := []string{
		"jimmy", "carl", "sheen", "cindy",
	}

	config := testnode.DefaultConfig().
		WithChainID(chainID).
		WithFundedAccounts(accounts...).
		WithDelayedPrecommitTimeout(time.Millisecond)

	cctx, _, grpcAddr := testnode.NewNetwork(t, config)
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0], localHeader{cctx.Client}, conn, chainID, nil,
		WithTxWorkerAccounts(8))
	require.NoError(t, err)
	err = ca.Start(ctx)
	require.NoError(t, err)

	randBlob, err := libshare.GenerateV0Blobs([]int{8}, false)
	require.NoError(t, err)

	nonDefaultAcct, err := cctx.Keyring.Key(accounts[3])
	require.NoError(t, err)
	addr, err := nonDefaultAcct.GetAddress()
	require.NoError(t, err)

	// check bals of accounts before tx (TODO @renaynay: hack til we get signer in txresp)
	sdkAddress, err := sdktypes.AccAddressFromHexUnsafe(fmt.Sprintf("%X", addr.Bytes()))
	require.NoError(t, err)
	balNonDefault, err := ca.BalanceForAddress(ctx, Address{sdkAddress})
	require.NoError(t, err)
	balDefault, err := ca.Balance(ctx)
	require.NoError(t, err)

	txConf := NewTxConfig(WithSignerAddress(addr.String()))
	_, err = ca.SubmitPayForBlob(ctx, randBlob, txConf)
	require.NoError(t, err)

	// ensure balance has decreased for non-default account
	updatedBalNonDefault, err := ca.BalanceForAddress(ctx, Address{sdkAddress})
	require.NoError(t, err)
	require.True(t, updatedBalNonDefault.Amount.LT(balNonDefault.Amount))

	// ensure balance remained same for default account
	updatedBalDefault, err := ca.Balance(ctx)
	require.NoError(t, err)
	require.True(t, updatedBalDefault.Equal(balDefault))

	// TODO @renaynay: once tx response contains signer, check signer here
}

func buildAccessor(t *testing.T, opts ...Option) (*CoreAccessor, []string) {
	t.Helper()
	accounts := []string{
		"jimmy", "carl", "sheen", "cindy",
	}

	config := testnode.DefaultConfig().
		WithChainID(chainID).
		WithFundedAccounts(accounts...).
		WithDelayedPrecommitTimeout(time.Millisecond * 50)

	cctx, _, grpcAddr := testnode.NewNetwork(t, config)

	_, err := cctx.WaitForHeight(int64(2))
	require.NoError(t, err)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0], nil, conn, chainID, nil, opts...)
	require.NoError(t, err)
	return ca, accounts
}
