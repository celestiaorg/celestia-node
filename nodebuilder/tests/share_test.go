//go:build share || integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
)

func TestShareModule(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Second*1))
	blobSize := 128
	libBlob, err := libshare.GenerateV0Blobs([]int{blobSize}, true)
	require.NoError(t, err)

	nodeBlob, err := blob.ToNodeBlobs(libBlob[0])
	require.NoError(t, err)

	bridge := sw.NewBridgeNode()
	require.NoError(t, bridge.Start(ctx))
	sw.SetBootstrapper(t, bridge)

	fullNode := sw.NewFullNode()
	require.NoError(t, fullNode.Start(ctx))

	lightNode := sw.NewLightNode()
	require.NoError(t, lightNode.Start(ctx))

	bridgeClient := getAdminClient(ctx, bridge, t)
	fullClient := getAdminClient(ctx, fullNode, t)
	lightClient := getAdminClient(ctx, lightNode, t)

	height, err := fullClient.Blob.Submit(ctx, nodeBlob, state.NewTxConfig())
	require.NoError(t, err)

	_, err = fullClient.Header.WaitForHeight(ctx, height)
	require.NoError(t, err)
	_, err = lightClient.Header.WaitForHeight(ctx, height)
	require.NoError(t, err)

	sampledBlob, err := fullClient.Blob.Get(ctx, height, nodeBlob[0].Namespace(), nodeBlob[0].Commitment)
	require.NoError(t, err)

	hdr, err := fullClient.Header.GetByHeight(ctx, height)
	require.NoError(t, err)

	coords, err := shwap.SampleCoordsFrom1DIndex(sampledBlob.Index(), len(hdr.DAH.RowRoots))
	require.NoError(t, err)

	blobAsShares, err := blob.BlobsToShares(sampledBlob)
	require.NoError(t, err)
	// different clients allow to test different getters that are used to get the data.
	clients := []*client.Client{lightClient, fullClient, bridgeClient}

	testCases := []struct {
		name string
		doFn func(t *testing.T)
	}{
		{
			name: "SharesAvailable",
			doFn: func(t *testing.T) {
				for _, client := range clients {
					err := client.Share.SharesAvailable(ctx, height)
					require.NoError(t, err)
				}
			},
		},
		{
			name: "GetShareQ1",
			doFn: func(t *testing.T) {
				for _, client := range clients {
					// compare the share from quadrant1 by its coordinate.
					// Additionally check that received share the same as the first share of the blob.
					sh, err := client.Share.GetShare(ctx, height, coords.Row, coords.Col)
					require.NoError(t, err)
					assert.Equal(t, blobAsShares[0], sh)
				}
			},
		},
		{
			name: "GetShareQ4",
			doFn: func(t *testing.T) {
				for _, client := range clients {
					_, err := client.Share.GetShare(ctx, height, len(hdr.DAH.RowRoots)-1, len(hdr.DAH.ColumnRoots)-1)
					require.NoError(t, err)
				}
			},
		},
		{
			name: "GetSamplesQ1",
			doFn: func(t *testing.T) {
				dah := hdr.DAH
				requestCoords := []shwap.SampleCoords{coords}
				for _, client := range clients {
					// request from the first quadrant using the blob coordinates.
					samples, err := client.Share.GetSamples(ctx, hdr, requestCoords)
					require.NoError(t, err)
					err = samples[0].Verify(dah, coords.Row, coords.Col)
					require.NoError(t, err)
					require.Equal(t, blobAsShares[0], samples[0].Share)
				}
			},
		},
		{
			name: "GetSamplesQ4",
			doFn: func(t *testing.T) {
				dah := hdr.DAH
				coords := shwap.SampleCoords{Row: len(dah.RowRoots) - 1, Col: len(dah.RowRoots) - 1}
				requestCoords := []shwap.SampleCoords{coords}
				for _, client := range clients {
					// getting the last sample from the eds(from quadrant 4).
					samples, err := client.Share.GetSamples(ctx, hdr, requestCoords)
					require.NoError(t, err)
					err = samples[0].Verify(dah, coords.Row, coords.Col)
					require.NoError(t, err)
				}
			},
		},
		{
			name: "GetEDS",
			doFn: func(t *testing.T) {
				for _, client := range clients {
					eds, err := client.Share.GetEDS(ctx, height)
					require.NoError(t, err)
					rawShares := eds.Row(uint(coords.Row))
					sh, err := libshare.FromBytes([][]byte{rawShares[coords.Col]})
					require.NoError(t, err)
					assert.Equal(t, blobAsShares[0], sh[0])
				}
			},
		},
		{
			name: "GetRowQ1",
			doFn: func(t *testing.T) {
				dah := hdr.DAH
				for _, client := range clients {
					// request row from the first half of the EDS(using the blob's coordinates).
					row, err := client.Share.GetRow(ctx, height, coords.Row)
					require.NoError(t, err)
					// verify row against the DAH.
					err = row.Verify(dah, coords.Row)
					require.NoError(t, err)
					shrs, err := row.Shares()
					require.NoError(t, err)
					// additionally compare shares
					assert.Equal(t, blobAsShares[0], shrs[coords.Col])
				}
			},
		},
		{
			name: "GetRowQ4",
			doFn: func(t *testing.T) {
				dah := hdr.DAH
				coords := shwap.SampleCoords{Row: len(dah.RowRoots) - 1, Col: len(dah.RowRoots) - 1}
				for _, client := range clients {
					// request the last row
					row, err := client.Share.GetRow(ctx, height, coords.Row)
					require.NoError(t, err)
					// verify against DAH
					err = row.Verify(dah, coords.Row)
					require.NoError(t, err)
				}
			},
		},
		{
			name: "GetNamespaceData",
			doFn: func(t *testing.T) {
				dah := hdr.DAH
				for _, client := range clients {
					// request data from the blob's namespace
					nsData, err := client.Share.GetNamespaceData(ctx, height, blobAsShares[0].Namespace())
					require.NoError(t, err)

					// verify against the DAH
					err = nsData.Verify(dah, blobAsShares[0].Namespace())
					require.NoError(t, err)

					b, err := libshare.ParseBlobs(nsData.Flatten())
					require.NoError(t, err)

					blb, err := blob.ToNodeBlobs(b[0])
					require.NoError(t, err)
					// compare commitments
					require.Equal(t, nodeBlob[0].Commitment, blb[0].Commitment)
				}
			},
		},
		{
			name: "GetRangeNamespaceData",
			doFn: func(t *testing.T) {
				dah := hdr.DAH
				blobLength, err := sampledBlob.Length()
				require.NoError(t, err)
				for _, client := range clients {
					rng, err := client.Share.GetRange(
						ctx,
						height,
						sampledBlob.Index(),
						sampledBlob.Index()+blobLength,
					)
					require.NoError(t, err)
					err = rng.Verify(dah.Hash())
					require.NoError(t, err)

					shrs := rng.Shares
					blbs, err := libshare.ParseBlobs(shrs)
					require.NoError(t, err)

					parsedBlob, err := blob.ToNodeBlobs(blbs...)
					require.NoError(t, err)
					require.Equal(t, nodeBlob[0].Commitment, parsedBlob[0].Commitment)
				}
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.doFn(t)
		})
	}
}
