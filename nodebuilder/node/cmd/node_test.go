package cmd

import (
	"context"
	"testing"

	"github.com/cristalhq/jwt/v5"
	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/node/mocks"
)

func TestRevokeTokenCommand(t *testing.T) {
	ctx := context.Background()
	key := []byte("test-secret-key-for-jwt-signing-32")
	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	require.NoError(t, err)
	jwtVerifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	require.NoError(t, err)
	verifier := authtoken.NewVerifier(jwtVerifier, ds_sync.MutexWrap(datastore.NewMapDatastore()))
	admin, err := authtoken.NewSignedJWT(signer, perms.AllPerms, 0)
	require.NoError(t, err)
	target, err := authtoken.NewSignedJWT(signer, perms.ReadPerms, 0)
	require.NoError(t, err)
	module := mocks.NewMockModule(gomock.NewController(t))
	module.EXPECT().AuthRevoke(gomock.Any(), target).Return(nil)
	server := rpc.NewServer("127.0.0.1", "0", false, rpc.CORSConfig{}, rpc.TLSConfig{},
		rpc.RateLimitConfig{}, signer, verifier)
	server.RegisterService("node", module, &node.API{})
	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() { require.NoError(t, server.Stop(ctx)) })
	Cmd.PersistentFlags().AddFlagSet(cmdnode.RPCFlags())
	Cmd.SetArgs([]string{"revoke-token", target, "--url", "http://" + server.ListenAddr(), "--token", admin})
	require.NoError(t, Cmd.ExecuteContext(ctx))
}
