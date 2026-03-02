//go:build api || integration

package tests

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/state"
)

const (
	btime = time.Millisecond * 300
)

func TestNodeModule(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))
	// start a bridge node
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	bridgeAddr := "http://" + bridge.RPCServer.ListenAddr()

	writePerms := []auth.Permission{"public", "read", "write"}
	adminPerms := []auth.Permission{"public", "read", "write", "admin"}
	jwt, err := bridge.AdminServ.AuthNew(ctx, adminPerms)
	require.NoError(t, err)

	client, err := client.NewClient(ctx, bridgeAddr, jwt)
	require.NoError(t, err)

	info, err := client.Node.Info(ctx)
	require.NoError(t, err)
	require.Equal(t, info.APIVersion, node.APIVersion)

	ready, err := client.Node.Ready(ctx)
	require.NoError(t, err)
	require.True(t, ready)

	perms, err := client.Node.AuthVerify(ctx, jwt)
	require.NoError(t, err)
	require.Equal(t, perms, adminPerms)

	writeJWT, err := client.Node.AuthNew(ctx, writePerms)
	require.NoError(t, err)

	perms, err = client.Node.AuthVerify(ctx, writeJWT)
	require.NoError(t, err)
	require.Equal(t, perms, writePerms)
}

func TestGetByHeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))

	// start a bridge node
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	rpcClient := getAdminClient(ctx, bridge, t)

	// let a few blocks be produced
	_, err = rpcClient.Header.WaitForHeight(ctx, 3)
	require.NoError(t, err)

	networkHead, err := rpcClient.Header.NetworkHead(ctx)
	require.NoError(t, err)
	_, err = rpcClient.Header.GetByHeight(ctx, networkHead.Height()+1)
	require.Nil(t, err, "Requesting syncer.Head()+1 shouldn't return an error")

	networkHead, err = rpcClient.Header.NetworkHead(ctx)
	require.NoError(t, err)
	_, err = rpcClient.Header.GetByHeight(ctx, networkHead.Height()+2)
	require.ErrorContains(t, err, "given height is from the future")
}

// TestBlobRPC ensures that blobs can be submitted via rpc
func TestBlobRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))

	// start a bridge node
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	rpcClient := getAdminClient(ctx, bridge, t)

	libBlobs, err := libshare.GenerateV0Blobs([]int{8}, false)
	require.NoError(t, err)

	newBlob, err := blob.NewBlob(
		libBlobs[0].ShareVersion(),
		libBlobs[0].Namespace(),
		libBlobs[0].Data(),
		libBlobs[0].Signer(),
	)
	require.NoError(t, err)

	height, err := rpcClient.Blob.Submit(ctx, []*blob.Blob{newBlob}, state.NewTxConfig())
	require.NoError(t, err)
	require.True(t, height != 0)

	txResp, err := rpcClient.State.SubmitPayForBlob(ctx, libBlobs, state.NewTxConfig())
	require.NoError(t, err)
	require.NotNil(t, txResp)
	require.Equal(t, uint32(0), txResp.Code)

	b, err := rpcClient.Blob.Get(ctx, uint64(txResp.Height), newBlob.Namespace(), newBlob.Commitment)
	require.NoError(t, err)
	require.NotNil(t, b)
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
	sw.SetBootstrapper(t, bridge)

	// start a light node that's connected to the bridge node
	light := sw.NewLightNode()
	err = light.Start(ctx)
	require.NoError(t, err)

	lightClient := getAdminClient(ctx, light, t)

	// subscribe to headers via the light node's RPC header subscription
	subctx, subcancel := context.WithCancel(ctx)
	sub, err := lightClient.Header.Subscribe(subctx)
	require.NoError(t, err)
	// listen for 5 headers
	for range 5 {
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

func TestSubmitBlobOverHTTP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))
	// start a bridge node
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	adminPerms := []auth.Permission{"public", "read", "write", "admin"}
	jwt, err := bridge.AdminServ.AuthNew(ctx, adminPerms)
	require.NoError(t, err)

	payload, err := os.ReadFile("testdata/submitPFB.json")
	require.NoError(t, err)

	bridgeAddr := "http://" + bridge.RPCServer.ListenAddr()
	req, err := http.NewRequest("POST", bridgeAddr, bytes.NewBuffer(payload))
	require.NoError(t, err)

	req.Header = http.Header{
		perms.AuthKey: []string{fmt.Sprintf("Bearer %s", jwt)},
	}

	httpClient := &http.Client{Timeout: time.Second * 5}
	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
