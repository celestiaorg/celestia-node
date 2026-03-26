package txclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v8/test/util/testnode"
)

func BuildClient(t *testing.T, chainID string, accounts []string, opts ...Option) *TxClient {
	t.Helper()
	config := testnode.DefaultConfig().
		WithChainID(chainID).
		WithFundedAccounts(accounts...).
		WithDelayedPrecommitTimeout(time.Millisecond * 50)

	cctx, _, grpcAddr := testnode.NewNetwork(t, config)

	_, err := cctx.WaitForHeight(int64(2))
	require.NoError(t, err)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	client, err := NewTxClient(cctx.Keyring, accounts[0], conn, opts...)
	require.NoError(t, err)
	return client
}
