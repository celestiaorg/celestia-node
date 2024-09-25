//go:build da || integration

package tests

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v3/pkg/appconsts"
	"github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/blob/blobtest"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

func TestDaModule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))

	namespace, err := share.NewV0Namespace([]byte("namespace"))
	require.NoError(t, err)
	require.False(t, namespace.IsReserved())

	appBlobs0, err := blobtest.GenerateV0Blobs([]int{8, 4}, true)
	require.NoError(t, err)
	appBlobs1, err := blobtest.GenerateV0Blobs([]int{4}, false)
	require.NoError(t, err)
	blobs := make([]*blob.Blob, 0, len(appBlobs0)+len(appBlobs1))
	daBlobs := make([][]byte, 0, len(appBlobs0)+len(appBlobs1))

	for _, squareBlob := range append(appBlobs0, appBlobs1...) {
		blob, err := blob.NewBlob(
			squareBlob.ShareVersion(),
			squareBlob.Namespace(),
			squareBlob.Data(),
			squareBlob.Signer(),
		)
		require.NoError(t, err)
		blobs = append(blobs, blob)
		daBlobs = append(daBlobs, blob.Data())
	}

	require.NoError(t, err)
	bridge := sw.NewBridgeNode()
	require.NoError(t, bridge.Start(ctx))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	fullCfg := sw.DefaultTestConfig(node.Full)
	fullCfg.Header.TrustedPeers = append(fullCfg.Header.TrustedPeers, addrs[0].String())
	fullNode := sw.NewNodeWithConfig(node.Full, fullCfg)
	require.NoError(t, fullNode.Start(ctx))

	addrsFull, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(fullNode.Host))
	require.NoError(t, err)

	lightCfg := sw.DefaultTestConfig(node.Light)
	lightCfg.Header.TrustedPeers = append(lightCfg.Header.TrustedPeers, addrsFull[0].String())
	lightNode := sw.NewNodeWithConfig(node.Light, lightCfg)
	require.NoError(t, lightNode.Start(ctx))

	fullClient := getAdminClient(ctx, fullNode, t)
	lightClient := getAdminClient(ctx, lightNode, t)

	ids, err := fullClient.DA.Submit(ctx, daBlobs, -1, namespace.Bytes())
	require.NoError(t, err)

	test := []struct {
		name string
		doFn func(t *testing.T)
	}{
		{
			name: "MaxBlobSize",
			doFn: func(t *testing.T) {
				mbs, err := fullClient.DA.MaxBlobSize(ctx)
				require.NoError(t, err)
				require.Equal(t, mbs, uint64(appconsts.DefaultMaxBytes))
			},
		},
		{
			name: "GetProofs + Validate",
			doFn: func(t *testing.T) {
				h, _ := da.SplitID(ids[0])
				lightClient.Header.WaitForHeight(ctx, h)
				proofs, err := lightClient.DA.GetProofs(ctx, ids, namespace.Bytes())
				require.NoError(t, err)
				require.NotEmpty(t, proofs)
				valid, err := fullClient.DA.Validate(ctx, ids, proofs, namespace.Bytes())
				require.NoError(t, err)
				for _, v := range valid {
					require.True(t, v)
				}
			},
		},
		{
			name: "GetIDs",
			doFn: func(t *testing.T) {
				height, _ := da.SplitID(ids[0])
				result, err := fullClient.DA.GetIDs(ctx, height, namespace.Bytes())
				require.NoError(t, err)
				require.EqualValues(t, ids, result.IDs)
				header, err := lightClient.Header.GetByHeight(ctx, height)
				require.NoError(t, err)
				require.EqualValues(t, header.Time(), result.Timestamp)
			},
		},
		{
			name: "Get",
			doFn: func(t *testing.T) {
				h, _ := da.SplitID(ids[0])
				lightClient.Header.WaitForHeight(ctx, h)
				fetched, err := lightClient.DA.Get(ctx, ids, namespace.Bytes())
				require.NoError(t, err)
				require.Len(t, fetched, len(ids))
				for i := range fetched {
					require.True(t, bytes.Equal(fetched[i], daBlobs[i]))
				}
			},
		},
		{
			name: "Commit",
			doFn: func(t *testing.T) {
				fetched, err := fullClient.DA.Commit(ctx, daBlobs, namespace.Bytes())
				require.NoError(t, err)
				require.Len(t, fetched, len(ids))
				for i := range fetched {
					_, commitment := da.SplitID(ids[i])
					require.EqualValues(t, fetched[i], commitment)
				}
			},
		},
		{
			name: "SubmitWithOptions - valid",
			doFn: func(t *testing.T) {
				ids, err := fullClient.DA.SubmitWithOptions(ctx, daBlobs, -1, namespace, []byte(`{"key_name": "validator"}`))
				require.NoError(t, err)
				require.NotEmpty(t, ids)
			},
		},
		{
			name: "SubmitWithOptions - invalid JSON",
			doFn: func(t *testing.T) {
				ids, err := fullClient.DA.SubmitWithOptions(ctx, daBlobs, -1, namespace, []byte("not JSON"))
				require.Error(t, err)
				require.Nil(t, ids)
			},
		},
		{
			name: "SubmitWithOptions - invalid key name",
			doFn: func(t *testing.T) {
				ids, err := fullClient.DA.SubmitWithOptions(ctx, daBlobs, -1, namespace, []byte(`{"key_name": "invalid"}`))
				require.Error(t, err)
				require.Nil(t, ids)
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			tt.doFn(t)
		})
	}
}
