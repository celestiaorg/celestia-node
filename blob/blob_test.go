package blob

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
)

func TestBlob(t *testing.T) {
	blob := GenerateBlobs(t, []int{1}, false)

	var test = []struct {
		name        string
		expectedRes func(t *testing.T)
	}{
		{
			name: "new blob",
			expectedRes: func(t *testing.T) {
				require.NotEmpty(t, blob)
				require.NotEmpty(t, blob[0].NamespaceID())
				require.NotEmpty(t, blob[0].Data())
				require.NotEmpty(t, blob[0].Commitment())
			},
		},
		{
			name: "compare commitments",
			expectedRes: func(t *testing.T) {
				comm, err := apptypes.CreateCommitment(&blob[0].Blob)
				require.NoError(t, err)
				assert.Equal(t, blob[0].Commitment(), Commitment(comm))
			},
		},
		{
			name: "verify nID",
			expectedRes: func(t *testing.T) {
				require.NoError(t, apptypes.ValidateBlobNamespaceID(blob[0].NamespaceID()))
			},
		},
		{
			name: "shares to blobs",
			expectedRes: func(t *testing.T) {
				sh, err := BlobsToShares(blob...)
				require.NoError(t, err)
				b, err := SharesToBlobs(sh)
				require.NoError(t, err)
				assert.Equal(t, len(b), 1)
				assert.Equal(t, blob[0].Commitment(), b[0].Commitment())
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, tt.expectedRes)
	}
}
