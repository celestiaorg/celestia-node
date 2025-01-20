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

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/state"
)

func TestBlobModule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second*1))

	libBlobs0, err := libshare.GenerateV0Blobs([]int{8, 4}, true)
	require.NoError(t, err)
	libBlobs1, err := libshare.GenerateV0Blobs([]int{4}, false)
	require.NoError(t, err)
	blobs := make([]*blob.Blob, 0, len(libBlobs0)+len(libBlobs1))

	for _, libBlob := range append(libBlobs0, libBlobs1...) {
		blob, err := convert(libBlob)
		require.NoError(t, err)
		blobs = append(blobs, blob)
	}

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

	address, err := fullClient.State.AccountAddress(ctx)
	require.NoError(t, err)
	v1Blob, err := libshare.NewV1Blob(
		libshare.MustNewV0Namespace(bytes.Repeat([]byte{5}, libshare.NamespaceVersionZeroIDSize)),
		[]byte("test data"),
		address.Bytes(),
	)
	require.NoError(t, err)

	v1, err := convert(v1Blob)
	require.NoError(t, err)
	blobs = append(blobs, v1)

	height, err := fullClient.Blob.Submit(ctx, blobs, state.NewTxConfig())
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
			name: "GetV0",
			doFn: func(t *testing.T) {
				blob1, err := fullClient.Blob.Get(ctx, height, blobs[0].Namespace(), blobs[0].Commitment)
				require.NoError(t, err)
				assert.Equal(t, blobs[0].Commitment, blob1.Commitment)
				assert.Equal(t, blobs[0].Data(), blob1.Data())
				assert.Nil(t, blob1.Signer())
			},
		},
		{
			name: "GetAllV0",
			doFn: func(t *testing.T) {
				newBlobs, err := fullClient.Blob.GetAll(ctx, height, []libshare.Namespace{blobs[0].Namespace()})
				require.NoError(t, err)
				assert.Len(t, newBlobs, len(libBlobs0))
				assert.Equal(t, blobs[0].Commitment, newBlobs[0].Commitment)
				assert.Equal(t, blobs[1].Commitment, newBlobs[1].Commitment)
				assert.Nil(t, newBlobs[0].Signer())
				assert.Nil(t, newBlobs[1].Signer())
			},
		},
		{
			name: "Get BlobV1",
			doFn: func(t *testing.T) {
				blobV1, err := fullClient.Blob.Get(ctx, height, v1.Namespace(), v1.Commitment)
				require.NoError(t, err)
				assert.Equal(t, libshare.ShareVersionOne, blobV1.ShareVersion())
				assert.Equal(t, v1.Commitment, blobV1.Commitment)
				assert.NotNil(t, blobV1.Signer())
				assert.Equal(t, blobV1.Signer(), v1.Signer())

			},
		},
		{
			name: "Included",
			doFn: func(t *testing.T) {
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
				libBlob, err := libshare.GenerateV0Blobs([]int{4}, false)
				require.NoError(t, err)
				newBlob, err := convert(libBlob[0])
				require.NoError(t, err)

				b, err := fullClient.Blob.Get(ctx, height, newBlob.Namespace(), newBlob.Commitment)
				assert.Nil(t, b)
				require.Error(t, err)
				require.ErrorContains(t, err, blob.ErrBlobNotFound.Error())

				blobs, err := fullClient.Blob.GetAll(ctx, height, []libshare.Namespace{newBlob.Namespace()})
				require.NoError(t, err)
				assert.Empty(t, blobs)
			},
		},
		{
			name: "Submit equal blobs",
			doFn: func(t *testing.T) {
				libBlob, err := libshare.GenerateV0Blobs([]int{8, 4}, true)
				require.NoError(t, err)
				b, err := convert(libBlob[0])
				require.NoError(t, err)

				height, err := fullClient.Blob.Submit(ctx, []*blob.Blob{b, b}, state.NewTxConfig())
				require.NoError(t, err)

				_, err = fullClient.Header.WaitForHeight(ctx, height)
				require.NoError(t, err)

				b0, err := fullClient.Blob.Get(ctx, height, b.Namespace(), b.Commitment)
				require.NoError(t, err)
				require.Equal(t, b.Commitment, b0.Commitment)

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
				h, err := fullClient.Blob.Submit(ctx, []*blob.Blob{blobs[0]}, state.NewTxConfig())
				require.NoError(t, err)

				_, err = fullClient.Header.WaitForHeight(ctx, h)
				require.NoError(t, err)

				b0, err := fullClient.Blob.Get(ctx, h, blobs[0].Namespace(), blobs[0].Commitment)
				require.NoError(t, err)
				require.Equal(t, blobs[0].Commitment, b0.Commitment)

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

// convert converts a libshare.Blob to a blob.Blob.
// convert may be deduplicated with convertBlobs from the blob package.
func convert(libBlob *libshare.Blob) (nodeBlob *blob.Blob, err error) {
	return blob.NewBlob(libBlob.ShareVersion(), libBlob.Namespace(), libBlob.Data(), libBlob.Signer())
}
