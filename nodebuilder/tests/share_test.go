//go:build share || integration

package tests

import (
	"context"
	"math"
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
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
)

func TestShareModule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second*1))
	blobSize := 128
	libBlob, err := libshare.GenerateV0Blobs([]int{blobSize}, true)
	require.NoError(t, err)

	nodeBlob, err := convert(libBlob[0])
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

	height, err := fullClient.Blob.Submit(ctx, []*blob.Blob{nodeBlob}, state.NewTxConfig())
	require.NoError(t, err)

	_, err = fullClient.Header.WaitForHeight(ctx, height)
	require.NoError(t, err)
	_, err = lightClient.Header.WaitForHeight(ctx, height)
	require.NoError(t, err)

	sampledBlob, err := fullClient.Blob.Get(ctx, height, nodeBlob.Namespace(), nodeBlob.Commitment)
	require.NoError(t, err)

	hdr, err := fullClient.Header.GetByHeight(ctx, height)
	require.NoError(t, err)

	coords, err := shwap.SampleCoordsFrom1DIndex(sampledBlob.Index(), len(hdr.DAH.RowRoots))
	require.NoError(t, err)

	blobAsShares, err := blob.BlobsToShares(sampledBlob)
	require.NoError(t, err)

	test := []struct {
		name string
		doFn func(t *testing.T)
	}{
		{
			name: "SharesAvailable",
			doFn: func(t *testing.T) {
				err := lightClient.Share.SharesAvailable(ctx, height)
				require.NoError(t, err)
			},
		},
		{
			name: "SharesAvailable_InvalidHeight",
			doFn: func(t *testing.T) {
				err := lightClient.Share.SharesAvailable(ctx, 0)
				require.Error(t, err)
			},
		},
		{
			name: "SharesAvailable_FutureHeight",
			doFn: func(t *testing.T) {
				err := lightClient.Share.SharesAvailable(ctx, math.MaxUint)
				require.Error(t, err)
			},
		},
		{
			name: "GetShare",
			doFn: func(t *testing.T) {
				sh, err := lightClient.Share.GetShare(ctx, height, coords.Row, coords.Col)
				require.NoError(t, err)
				assert.Equal(t, blobAsShares[0], sh)
			},
		},
		{
			name: "GetShare_InvalidRow",
			doFn: func(t *testing.T) {
				sh, err := lightClient.Share.GetShare(ctx, height, -1, coords.Col)
				require.Error(t, err)
				assert.Nil(t, sh.ToBytes())
			},
		},
		{
			name: "GetShare_InvalidCol",
			doFn: func(t *testing.T) {
				sh, err := lightClient.Share.GetShare(ctx, height, coords.Row, -1)
				require.Error(t, err)
				assert.Nil(t, sh.ToBytes())
			},
		},
		{
			name: "GetShare_InvalidCoords",
			doFn: func(t *testing.T) {
				dah := hdr.DAH
				sh, err := lightClient.Share.GetShare(ctx, height, len(dah.RowRoots), len(dah.ColumnRoots))
				require.Error(t, err)
				assert.Nil(t, sh.ToBytes())
			},
		},
		{
			name: "GetSamples",
			doFn: func(t *testing.T) {
				dah := hdr.DAH
				samples, err := lightClient.Share.GetSamples(ctx, hdr, []shwap.SampleCoords{coords})
				require.NoError(t, err)
				err = samples[0].Verify(dah, coords.Row, coords.Col)
				require.NoError(t, err)
			},
		},
		{
			name: "GetSamples_InvalidCoords",
			doFn: func(t *testing.T) {
				dah := hdr.DAH
				coords := shwap.SampleCoords{Row: len(dah.RowRoots), Col: len(dah.ColumnRoots)}
				samples, err := lightClient.Share.GetSamples(ctx, hdr, []shwap.SampleCoords{coords})
				require.Error(t, err)
				assert.Nil(t, samples)
			},
		},
		{
			name: "GetEDS",
			doFn: func(t *testing.T) {
				eds, err := lightClient.Share.GetEDS(ctx, height)
				require.NoError(t, err)
				rawShares := eds.Row(uint(coords.Row))
				sh, err := libshare.FromBytes([][]byte{rawShares[coords.Col]})
				require.NoError(t, err)
				assert.Equal(t, blobAsShares[0], sh[0])
			},
		},
		{
			name: "GetRow",
			doFn: func(t *testing.T) {
				row, err := lightClient.Share.GetRow(ctx, height, coords.Row)
				require.NoError(t, err)
				dah := hdr.DAH
				err = row.Verify(dah, coords.Row)
				require.NoError(t, err)

				shrs, err := row.Shares()
				require.NoError(t, err)
				assert.Equal(t, blobAsShares[0], shrs[coords.Col])
			},
		},
		{
			name: "GetRow_InvalidRow",
			doFn: func(t *testing.T) {
				_, err := lightClient.Share.GetRow(ctx, height, -1)
				require.Error(t, err)
				_, err = lightClient.Share.GetRow(ctx, height, math.MinInt64)
				require.Error(t, err)
			},
		},
		{
			name: "GetNamespaceData",
			doFn: func(t *testing.T) {
				nsData, err := lightClient.Share.GetNamespaceData(ctx, height, blobAsShares[0].Namespace())
				require.NoError(t, err)
				dah := hdr.DAH
				err = nsData.Verify(dah, blobAsShares[0].Namespace())
				require.NoError(t, err)
				blob, err := libshare.ParseBlobs(nsData.Flatten())
				require.NoError(t, err)
				blb, err := convert(blob[0])
				require.NoError(t, err)
				require.Equal(t, nodeBlob.Commitment, blb.Commitment)
			},
		},
	}

	for _, tt := range test {
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
