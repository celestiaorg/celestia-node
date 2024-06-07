package blob

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	v1 "github.com/celestiaorg/celestia-app/pkg/appconsts/v1"
	apptypes "github.com/celestiaorg/celestia-app/v2/x/blob/types"
	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/inclusion"
	"github.com/celestiaorg/go-square/merkle"

	"github.com/celestiaorg/celestia-node/blob/blobtest"
)

func TestBlob(t *testing.T) {
	appBlobs, err := blobtest.GenerateV0Blobs([]int{16}, false)
	require.NoError(t, err)
	blob, err := convertBlobs(appBlobs...)
	require.NoError(t, err)

	test := []struct {
		name        string
		expectedRes func(t *testing.T)
	}{
		{
			name: "new blob",
			expectedRes: func(t *testing.T) {
				require.NotEmpty(t, blob)
				require.NotEmpty(t, blob[0].Namespace())
				require.NotEmpty(t, blob[0].Data)
				require.NotEmpty(t, blob[0].Commitment)
			},
		},
		{
			name: "compare commitments",
			expectedRes: func(t *testing.T) {
				comm, err := inclusion.CreateCommitment(
					blob[0].Blob,
					merkle.HashFromByteSlices,
					appconsts.SubtreeRootThreshold(v1.Version),
				)
				require.NoError(t, err)
				assert.Equal(t, blob[0].Commitment, Commitment(comm))
			},
		},
		{
			name: "verify namespace",
			expectedRes: func(t *testing.T) {
				ns := blob[0].Namespace().ToAppNamespace()
				require.NoError(t, err)
				require.NoError(t, apptypes.ValidateBlobNamespace(ns))
			},
		},
		{
			name: "shares to blobs",
			expectedRes: func(t *testing.T) {
				sh, err := BlobsToShares(blob...)
				require.NoError(t, err)
				shares, err := toAppShares(sh...)
				require.NoError(t, err)
				p := &parser{length: len(shares), shares: shares}
				b, err := p.parse()
				require.NoError(t, err)
				assert.Equal(t, blob[0].Commitment, b.Commitment)
			},
		},
		{
			name: "blob marshaling",
			expectedRes: func(t *testing.T) {
				data, err := blob[0].MarshalJSON()
				require.NoError(t, err)

				newBlob := &Blob{}
				require.NoError(t, newBlob.UnmarshalJSON(data))
				require.True(t, bytes.Equal(blob[0].Blob.GetData(), newBlob.Data))
				require.True(t, bytes.Equal(blob[0].Commitment, newBlob.Commitment))
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, tt.expectedRes)
	}
}

func convertBlobs(appBlobs ...*blob.Blob) ([]*Blob, error) {
	blobs := make([]*Blob, 0, len(appBlobs))
	for _, appBlob := range appBlobs {
		if shareVersion := appBlob.GetShareVersion(); shareVersion > math.MaxUint8 {
			return nil, fmt.Errorf("share version %d is greater than max share version %d", shareVersion, math.MaxUint8)
		}
		blob, err := NewBlob(uint8(appBlob.GetShareVersion()), appBlob.Namespace().Bytes(), appBlob.GetData())
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, blob)
	}
	return blobs, nil
}
