package tests

import (
	"context"
	"testing"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
	"github.com/celestiaorg/celestia-node/nodebuilder"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/stretchr/testify/require"
)

//nolint:unused
func getAdminClient(ctx context.Context, nd *nodebuilder.Node, t *testing.T) *client.Client {
	t.Helper()

	signer := nd.AdminSigner
	listenAddr := "ws://" + nd.RPCServer.ListenAddr()

	jwt, err := authtoken.NewSignedJWT(signer, []auth.Permission{"public", "read", "write", "admin"})
	require.NoError(t, err)

	client, err := client.NewClient(ctx, listenAddr, jwt)
	require.NoError(t, err)

	return client
}
