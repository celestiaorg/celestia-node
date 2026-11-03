package node

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
)

func TestAuthRevokeRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key := []byte("test-secret-key-for-jwt-signing-32")
	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	require.NoError(t, err)
	jwtVerifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	require.NoError(t, err)
	verifier := authtoken.NewVerifier(jwtVerifier, ds_sync.MutexWrap(datastore.NewMapDatastore()))
	module := newModule(Light, signer, verifier)
	server := rpc.NewServer("127.0.0.1", "0", false, rpc.CORSConfig{}, rpc.TLSConfig{},
		rpc.RateLimitConfig{}, signer, verifier)
	server.RegisterService("node", module, &API{})
	server.RegisterService("test", revocationSubscription{}, &revocationSubscriptionAPI{})
	require.NoError(t, server.WithMetrics())
	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() { require.NoError(t, server.Stop(context.Background())) })

	adminToken, err := module.AuthNew(ctx, perms.AllPerms)
	require.NoError(t, err)
	targetToken, err := module.AuthNewWithExpiry(ctx, perms.ReadPerms, time.Hour)
	require.NoError(t, err)
	unaffectedToken, err := authtoken.NewSignedJWT(signer, perms.ReadPerms, 0)
	require.NoError(t, err)
	connect := func(token string) *API {
		t.Helper()
		var api API
		closer, err := jsonrpc.NewClient(ctx, "http://"+server.ListenAddr(), "node", &api.Internal,
			http.Header{"Authorization": []string{"Bearer " + token}})
		require.NoError(t, err)
		t.Cleanup(closer)
		return &api
	}
	admin := connect(adminToken)
	target := connect(targetToken)
	unaffected := connect(unaffectedToken)
	ready, err := target.Ready(ctx)
	require.NoError(t, err)
	require.True(t, ready)
	require.ErrorContains(t, target.AuthRevoke(ctx, adminToken), "missing permission")

	var subscription struct {
		Updates func(context.Context) (<-chan int, error)
	}
	// Query-string authentication is supported for browser WebSocket clients too.
	closer, err := jsonrpc.NewClient(ctx, "ws://"+server.ListenAddr()+"?token="+targetToken,
		"test", &subscription, nil)
	require.NoError(t, err)
	t.Cleanup(closer)
	updates, err := subscription.Updates(ctx)
	require.NoError(t, err)
	select {
	case value := <-updates:
		require.Equal(t, 1, value)
	case <-ctx.Done():
		t.Fatal("subscription did not start")
	}
	require.NoError(t, admin.AuthRevoke(ctx, targetToken))
	require.NoError(t, admin.AuthRevoke(ctx, targetToken))
	select {
	case _, ok := <-updates:
		require.False(t, ok, "revocation must close the existing subscription")
	case <-ctx.Done():
		t.Fatal("subscription remained open after revocation")
	}
	_, err = jsonrpc.NewClient(ctx, "ws://"+server.ListenAddr()+"?token="+targetToken,
		"test", &subscription, nil)
	require.Error(t, err, "revoked token must not reconnect")
	_, err = target.Ready(ctx)
	require.Error(t, err)
	_, err = admin.AuthVerify(ctx, targetToken)
	require.ErrorContains(t, err, "token revoked")
	require.Error(t, target.AuthRevoke(ctx, adminToken))
	ready, err = unaffected.Ready(ctx)
	require.NoError(t, err)
	require.True(t, ready)
	_, err = admin.Info(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.AuthRevoke(ctx, adminToken))
	_, err = admin.Info(ctx)
	require.Error(t, err)
}

type revocationSubscription struct{}

func (revocationSubscription) Updates(ctx context.Context) (<-chan int, error) {
	ch := make(chan int, 1)
	ch <- 1
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

type revocationSubscriptionAPI struct {
	Internal struct {
		Updates func(context.Context) (<-chan int, error) `perm:"read"`
	}
}

func (a *revocationSubscriptionAPI) Updates(ctx context.Context) (<-chan int, error) {
	return a.Internal.Updates(ctx)
}
