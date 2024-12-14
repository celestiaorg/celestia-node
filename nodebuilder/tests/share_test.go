//go:build share || integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
)

func TestShareModule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
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

	odsIndex, err := shwap.SampleCoordsAs1DIndex(coords, len(hdr.DAH.RowRoots)/2)
	require.NoError(t, err)

	blobAsShares, err := blob.BlobsToShares(sampledBlob)
	require.NoError(t, err)
	length, err := sampledBlob.Length()
	require.NoError(t, err)
	test := []struct {
		name string
		doFn func(t *testing.T)
	}{
		{
			name: "GetShare",
			doFn: func(t *testing.T) {
				sh, err := lightClient.Share.GetShare(ctx, height, coords.Row, coords.Col)
				require.NoError(t, err)
				assert.Equal(t, blobAsShares[0], sh)
			},
		},
		{
			name: "GetRange_Full",
			doFn: func(t *testing.T) {
				rng, err := lightClient.Share.GetRange(
					ctx, nodeBlob.Namespace(), height, uint32(odsIndex), uint32(odsIndex+length)-1, false,
				)
				require.NoError(t, err)
				assert.Len(t, rng.Shares, len(blobAsShares))
				require.NoError(t, rng.Proof.Validate(hdr.DataHash))
			},
		},
		{
			name: "GetRange_Partial",
			doFn: func(t *testing.T) {
				offset := rand.Intn(length - 1)
				from := odsIndex + offset
				to := odsIndex + length - 1 - offset
				if from > to {
					from, to = to, from
				}
				rng, err := lightClient.Share.GetRange(
					ctx, nodeBlob.Namespace(), height, uint32(from), uint32(to), false,
				)
				require.NoError(t, err)
				expectedLength := to - from + 1
				assert.Len(t, rng.Shares, expectedLength)
				require.NoError(t, rng.Proof.Validate(hdr.DataHash))
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
