package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/blob/blobtest"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

// TestBlobRPC ensures that blobs can be submited via rpc
func TestBlobRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))

	// start a bridge node
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	signer := bridge.AdminSigner
	bridgeAddr := "http://" + bridge.RPCServer.ListenAddr()
	fmt.Println("bridgeAddr: ", bridgeAddr)

	jwt, err := authtoken.NewSignedJWT(signer, []auth.Permission{"public", "read", "write", "admin"})
	require.NoError(t, err)

	client, err := client.NewClient(ctx, bridgeAddr, jwt)
	require.NoError(t, err)

	appBlobs, err := blobtest.GenerateBlobs([]int{8}, false)
	require.NoError(t, err)

	newBlob, err := blob.NewBlob(
		appBlobs[0].ShareVersion,
		append([]byte{appBlobs[0].NamespaceVersion}, appBlobs[0].NamespaceID...),
		appBlobs[0].Data,
	)
	require.NoError(t, err)

	height, err := client.Blob.Submit(ctx, []*blob.Blob{newBlob})
	require.NoError(t, err)
	require.True(t, height != 0)
}

// TestHeaderSubscription ensures that the header subscription over RPC works
// as intended and gets canceled successfully after rpc context cancellation.
func TestHeaderSubscription(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))

	// start a bridge node
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Light)
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())

	// start a light node that's connected to the bridge node
	light := sw.NewNodeWithConfig(node.Light, cfg)
	err = light.Start(ctx)
	require.NoError(t, err)

	// subscribe to headers via the light node's RPC header subscription
	subctx, subcancel := context.WithCancel(ctx)
	sub, err := light.HeaderServ.Subscribe(subctx)
	require.NoError(t, err)
	// listen for 5 headers
	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-sub:
		}
	}
	// cancel subscription via context
	subcancel()

	// stop the light node and expect no outstanding subscription errors
	err = light.Stop(ctx)
	require.NoError(t, err)
}
