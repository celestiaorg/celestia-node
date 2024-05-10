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

	"github.com/celestiaorg/celestia-app/pkg/appconsts"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/blob/blobtest"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share"
)

func TestDaModule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))

	namespace, err := share.NewBlobNamespaceV0([]byte("namespace"))
	require.NoError(t, err)

	appBlobs0, err := blobtest.GenerateV0Blobs([]int{8, 4}, true)
	require.NoError(t, err)
	appBlobs1, err := blobtest.GenerateV0Blobs([]int{4}, false)
	require.NoError(t, err)
	blobs := make([]*blob.Blob, 0, len(appBlobs0)+len(appBlobs1))
	daBlobs := make([][]byte, 0, len(appBlobs0)+len(appBlobs1))

	for _, b := range append(appBlobs0, appBlobs1...) {
		blob, err := blob.NewBlob(b.ShareVersion, namespace, b.Data)
		require.NoError(t, err)
		blobs = append(blobs, blob)
		daBlobs = append(daBlobs, blob.Data)
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

	ids, err := fullClient.DA.Submit(ctx, daBlobs, -1, namespace)
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
				t.Skip()
				h, _ := da.SplitID(ids[0])
				lightClient.Header.WaitForHeight(ctx, h)
				proofs, err := lightClient.DA.GetProofs(ctx, ids, namespace)
				require.NoError(t, err)
				require.NotEmpty(t, proofs)
				valid, err := fullClient.DA.Validate(ctx, ids, proofs, namespace)
				require.NoError(t, err)
				for _, v := range valid {
					require.True(t, v)
				}
			},
		},
		{
			name: "GetIDs",
			doFn: func(t *testing.T) {
				t.Skip()
				height, _ := da.SplitID(ids[0])
				ids2, err := fullClient.DA.GetIDs(ctx, height, namespace)
				require.NoError(t, err)
				require.EqualValues(t, ids, ids2)
			},
		},
		{
			name: "Get",
			doFn: func(t *testing.T) {
				h, _ := da.SplitID(ids[0])
				lightClient.Header.WaitForHeight(ctx, h)
				fetched, err := lightClient.DA.Get(ctx, ids, namespace)
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
				t.Skip()
				fetched, err := fullClient.DA.Commit(ctx, ids, namespace)
				require.NoError(t, err)
				require.Len(t, fetched, len(ids))
				for i := range fetched {
					_, commitment := da.SplitID(ids[i])
					require.EqualValues(t, fetched[i], commitment)
				}
			},
		},
	}

	for _, tt := range test {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.doFn(t)
		})
	}
}
