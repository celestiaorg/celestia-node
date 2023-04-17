package blob

import (
	"github.com/celestiaorg/celestia-app/pkg/shares"
)

// sharesToBlobs takes raw shares and converts them to blobs.
func sharesToBlobs(rawShares [][]byte) ([]*Blob, error) {
	if len(rawShares) == 0 {
		return nil, errBlobNotFound
	}

	shareSequences, err := shares.ParseShares(rawShares)
	if err != nil {
		return nil, err
	}

	blobs := make([]*Blob, len(shareSequences))
	for i, sequence := range shareSequences {
		data, err := sequence.RawData()
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			continue
		}

		blob, err := NewBlob(data[0], sequence.NamespaceID, data)
		if err != nil {
			return nil, err
		}
		blobs[i] = blob
	}
	return blobs, nil
}
