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

	"github.com/celestiaorg/celestia-app/v7/pkg/appconsts"
	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

func TestDaModule(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second))

	namespace, err := libshare.NewV0Namespace([]byte("namespace"))
	require.NoError(t, err)
	require.False(t, namespace.IsReserved())

	libBlobs0, err := libshare.GenerateV0Blobs([]int{8, 4}, true)
	require.NoError(t, err)
	libBlobs1, err := libshare.GenerateV0Blobs([]int{4}, false)
	require.NoError(t, err)
	blobs := make([]*blob.Blob, 0, len(libBlobs0)+len(libBlobs1))
	daBlobs := make([][]byte, 0, len(libBlobs0)+len(libBlobs1))

	for _, libBlob := range append(libBlobs0, libBlobs1...) {
		blob, err := blob.NewBlob(
			libBlob.ShareVersion(),
			libBlob.Namespace(),
			libBlob.Data(),
			libBlob.Signer(),
		)
		require.NoError(t, err)
		blobs = append(blobs, blob)
		daBlobs = append(daBlobs, blob.Data())
	}

	require.NoError(t, err)
	bridge := sw.NewBridgeNode()
	require.NoError(t, bridge.Start(ctx))
	sw.SetBootstrapper(t, bridge)

	fullNode := sw.NewBridgeNode()
	require.NoError(t, fullNode.Start(ctx))
	addrFn := host.InfoFromHost(fullNode.Host)

	lightNode := sw.NewLightNode(nodebuilder.WithBootstrappers([]peer.AddrInfo{*addrFn}))
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
				ids, err := fullClient.DA.SubmitWithOptions(ctx, daBlobs, -1, namespace.Bytes(), []byte(`{"key_name": "validator"}`))
				require.NoError(t, err)
				require.NotEmpty(t, ids)
			},
		},
		{
			name: "SubmitWithOptions - invalid JSON",
			doFn: func(t *testing.T) {
				ids, err := fullClient.DA.SubmitWithOptions(ctx, daBlobs, -1, namespace.Bytes(), []byte("not JSON"))
				require.Error(t, err)
				require.Nil(t, ids)
			},
		},
		{
			name: "SubmitWithOptions - invalid key name",
			doFn: func(t *testing.T) {
				ids, err := fullClient.DA.SubmitWithOptions(ctx, daBlobs, -1, namespace.Bytes(), []byte(`{"key_name": "invalid"}`))
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
