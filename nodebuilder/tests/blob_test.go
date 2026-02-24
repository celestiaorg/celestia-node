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

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/state"
)

func TestBlobModule(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second*1))

	libBlobs0, err := libshare.GenerateV0Blobs([]int{8, 4}, true)
	require.NoError(t, err)
	libBlobs1, err := libshare.GenerateV0Blobs([]int{4}, false)
	require.NoError(t, err)

	blobs, err := blob.ToNodeBlobs(append(libBlobs0, libBlobs1...)...)
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

	address, err := fullClient.State.AccountAddress(ctx)
	require.NoError(t, err)
	v1Blob, err := libshare.NewV1Blob(
		libshare.MustNewV0Namespace(bytes.Repeat([]byte{5}, libshare.NamespaceVersionZeroIDSize)),
		[]byte("test data"),
		address.Bytes(),
	)
	require.NoError(t, err)

	v1, err := blob.ToNodeBlobs(v1Blob)
	require.NoError(t, err)
	blobs = append(blobs, v1[0])

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
				blobV1, err := fullClient.Blob.Get(ctx, height, v1[0].Namespace(), v1[0].Commitment)
				require.NoError(t, err)
				assert.Equal(t, libshare.ShareVersionOne, blobV1.ShareVersion())
				assert.Equal(t, v1[0].Commitment, blobV1.Commitment)
				assert.NotNil(t, blobV1.Signer())
				assert.Equal(t, blobV1.Signer(), v1[0].Signer())
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
				newBlob, err := blob.ToNodeBlobs(libBlob[0])
				require.NoError(t, err)

				b, err := fullClient.Blob.Get(ctx, height, newBlob[0].Namespace(), newBlob[0].Commitment)
				assert.Nil(t, b)
				require.Error(t, err)
				require.ErrorContains(t, err, blob.ErrBlobNotFound.Error())

				blobs, err := fullClient.Blob.GetAll(ctx, height, []libshare.Namespace{newBlob[0].Namespace()})
				require.NoError(t, err)
				assert.Empty(t, blobs)
			},
		},
		{
			name: "Submit equal blobs",
			doFn: func(t *testing.T) {
				libBlob, err := libshare.GenerateV0Blobs([]int{8, 4}, true)
				require.NoError(t, err)
				b, err := blob.ToNodeBlobs(libBlob[0])
				require.NoError(t, err)

				height, err := fullClient.Blob.Submit(ctx, []*blob.Blob{b[0], b[0]}, state.NewTxConfig())
				require.NoError(t, err)

				_, err = fullClient.Header.WaitForHeight(ctx, height)
				require.NoError(t, err)

				b0, err := fullClient.Blob.Get(ctx, height, b[0].Namespace(), b[0].Commitment)
				require.NoError(t, err)
				require.Equal(t, b[0].Commitment, b0.Commitment)

				proof, err := fullClient.Blob.GetProof(ctx, height, b[0].Namespace(), b[0].Commitment)
				require.NoError(t, err)

				included, err := fullClient.Blob.Included(ctx, height, b[0].Namespace(), proof, b[0].Commitment)
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
