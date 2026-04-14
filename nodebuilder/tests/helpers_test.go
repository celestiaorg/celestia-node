// this directive needs to be here and i wanted to remove but couldn't
// its too long to explain so just trust me or try removing yourself :)
//
//nolint:unused
package tests

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
	"github.com/celestiaorg/celestia-node/nodebuilder"
)

func getAdminClient(ctx context.Context, nd *nodebuilder.Node, t *testing.T) *client.Client {
	t.Helper()

	signer := nd.AdminSigner
	listenAddr := "ws://" + nd.RPCServer.ListenAddr()

	jwt, err := authtoken.NewSignedJWT(signer, []auth.Permission{"public", "read", "write", "admin"}, time.Minute)
	require.NoError(t, err)

	client, err := client.NewClient(context.WithoutCancel(ctx), listenAddr, jwt)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	return client
}

func setTimeInterval(cfg *nodebuilder.Config, interval time.Duration) {
	cfg.Share.Discovery.AdvertiseInterval = interval
}
