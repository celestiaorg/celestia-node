//go:build !race

package state

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v6/test/util/testnode"
	apptypes "github.com/celestiaorg/celestia-app/v6/x/blob/types"
	libshare "github.com/celestiaorg/go-square/v3/share"
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

func TestParallelPayForBlobSubmission(t *testing.T) {
	const (
		workerAccounts = 4
		blobCount      = 10
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	chainID := "private"

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

	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0], nil, conn, chainID, WithTxWorkerAccounts(workerAccounts))
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

func buildAccessor(t *testing.T, opts ...Option) (*CoreAccessor, []string) {
	chainID := "private"

	t.Helper()
	accounts := []string{
		"jimmy", "carl", "sheen", "cindy",
	}

	config := testnode.DefaultConfig().
		WithChainID(chainID).
		WithFundedAccounts(accounts...).
		WithTimeoutCommit(time.Millisecond * 1)

	cctx, _, grpcAddr := testnode.NewNetwork(t, config)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	ca, err := NewCoreAccessor(cctx.Keyring, accounts[0], nil, conn, chainID, opts...)
	require.NoError(t, err)
	return ca, accounts
}
