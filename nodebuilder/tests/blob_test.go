//go:build blob || integration

package tests

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/blob/blobtest"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share"
)

func TestBlobModule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second*1))

	appBlobs0, err := blobtest.GenerateV0Blobs([]int{8, 4}, true)
	require.NoError(t, err)
	appBlobs1, err := blobtest.GenerateV0Blobs([]int{4}, false)
	require.NoError(t, err)
	blobs := make([]*blob.Blob, 0, len(appBlobs0)+len(appBlobs1))

	for _, b := range append(appBlobs0, appBlobs1...) {
		blob, err := blob.NewBlob(b.ShareVersion, append([]byte{b.NamespaceVersion}, b.NamespaceID...), b.Data)
		require.NoError(t, err)
		blobs = append(blobs, blob)
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

	height, err := fullClient.Blob.Submit(ctx, blobs, blob.DefaultGasPrice())
	require.NoError(t, err)

	_, err = fullClient.Header.WaitForHeight(ctx, height)
	require.NoError(t, err)
	_, err = lightClient.Header.WaitForHeight(ctx, height)
	require.NoError(t, err)

	test := []struct {
		name string
		doFn func(t *testing.T)
	}{
		{
			name: "Get",
			doFn: func(t *testing.T) {
				// https://github.com/celestiaorg/celestia-node/issues/2915
				time.Sleep(time.Second)
				blob1, err := fullClient.Blob.Get(ctx, height, blobs[0].Namespace(), blobs[0].Commitment)
				require.NoError(t, err)
				require.Equal(t, blobs[0].Commitment, blob1.Commitment)
			},
		},
		{
			name: "GetAll",
			doFn: func(t *testing.T) {
				// https://github.com/celestiaorg/celestia-node/issues/2915
				time.Sleep(time.Second)
				newBlobs, err := fullClient.Blob.GetAll(ctx, height, []share.Namespace{blobs[0].Namespace()})
				require.NoError(t, err)
				require.Len(t, newBlobs, len(appBlobs0))
				require.True(t, bytes.Equal(blobs[0].Commitment, newBlobs[0].Commitment))
				require.True(t, bytes.Equal(blobs[1].Commitment, newBlobs[1].Commitment))
			},
		},
		{
			name: "Included",
			doFn: func(t *testing.T) {
				// https://github.com/celestiaorg/celestia-node/issues/2915
				time.Sleep(time.Second)
				proof, err := fullClient.Blob.GetProof(ctx, height, blobs[0].Namespace(), blobs[0].Commitment)
				require.NoError(t, err)

				included, err := lightClient.Blob.Included(
					ctx,
					height,
					blobs[0].Namespace(),
					proof,
					blobs[0].Commitment,
				)
				require.NoError(t, err)
				require.True(t, included)
			},
		},
		{
			name: "Not Found",
			doFn: func(t *testing.T) {
				appBlob, err := blobtest.GenerateV0Blobs([]int{4}, false)
				require.NoError(t, err)
				newBlob, err := blob.NewBlob(
					appBlob[0].ShareVersion,
					append([]byte{appBlob[0].NamespaceVersion}, appBlob[0].NamespaceID...),
					appBlob[0].Data,
				)
				require.NoError(t, err)

				b, err := fullClient.Blob.Get(ctx, height, newBlob.Namespace(), newBlob.Commitment)
				assert.Nil(t, b)
				require.Error(t, err)
				require.ErrorContains(t, err, blob.ErrBlobNotFound.Error())
			},
		},
		{
			name: "Submit equal blobs",
			doFn: func(t *testing.T) {
				appBlob, err := blobtest.GenerateV0Blobs([]int{8, 4}, true)
				require.NoError(t, err)
				b, err := blob.NewBlob(
					appBlob[0].ShareVersion,
					append([]byte{appBlob[0].NamespaceVersion}, appBlob[0].NamespaceID...),
					appBlob[0].Data,
				)
				require.NoError(t, err)

				height, err := fullClient.Blob.Submit(ctx, []*blob.Blob{b, b}, blob.DefaultGasPrice())
				require.NoError(t, err)

				_, err = fullClient.Header.WaitForHeight(ctx, height)
				require.NoError(t, err)

				b0, err := fullClient.Blob.Get(ctx, height, b.Namespace(), b.Commitment)
				require.NoError(t, err)
				require.Equal(t, b.Commitment, b0.Commitment)

				// give some time to store the data,
				// otherwise the test will hang on the IPLD level.
				// https://github.com/celestiaorg/celestia-node/issues/2915
				time.Sleep(time.Second)

				proof, err := fullClient.Blob.GetProof(ctx, height, b.Namespace(), b.Commitment)
				require.NoError(t, err)

				included, err := fullClient.Blob.Included(ctx, height, b.Namespace(), proof, b.Commitment)
				require.NoError(t, err)
				require.True(t, included)
			},
		},
		{
			// This test allows to check that the blob won't be
			// deduplicated if it will be sent multiple times in
			// different pfbs.
			name: "Submit the same blob in different pfb",
			doFn: func(t *testing.T) {
				h, err := fullClient.Blob.Submit(ctx, []*blob.Blob{blobs[0]}, blob.DefaultGasPrice())
				require.NoError(t, err)

				_, err = fullClient.Header.WaitForHeight(ctx, h)
				require.NoError(t, err)

				b0, err := fullClient.Blob.Get(ctx, h, blobs[0].Namespace(), blobs[0].Commitment)
				require.NoError(t, err)
				require.Equal(t, blobs[0].Commitment, b0.Commitment)

				// give some time to store the data,
				// otherwise the test will hang on the IPLD level.
				// https://github.com/celestiaorg/celestia-node/issues/2915
				time.Sleep(time.Second)

				proof, err := fullClient.Blob.GetProof(ctx, h, blobs[0].Namespace(), blobs[0].Commitment)
				require.NoError(t, err)

				included, err := fullClient.Blob.Included(ctx, h, blobs[0].Namespace(), proof, blobs[0].Commitment)
				require.NoError(t, err)
				require.True(t, included)
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
