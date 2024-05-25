package blob

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/types"

	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"

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
				comm, err := apptypes.CreateCommitment(&blob[0].Blob)
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
				require.True(t, bytes.Equal(blob[0].Blob.Data, newBlob.Data))
				require.True(t, bytes.Equal(blob[0].Commitment, newBlob.Commitment))
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, tt.expectedRes)
	}
}

func convertBlobs(appBlobs ...types.Blob) ([]*Blob, error) {
	blobs := make([]*Blob, 0, len(appBlobs))
	for _, b := range appBlobs {
		blob, err := NewBlob(b.ShareVersion, append([]byte{b.NamespaceVersion}, b.NamespaceID...), b.Data)
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, blob)
	}
	return blobs, nil
}
