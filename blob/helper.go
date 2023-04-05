package blob

import (
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-app/pkg/shares"

	"github.com/celestiaorg/celestia-node/share/ipld"
)

// sharesToBlobs takes raw shares and converts them to blobs.
func sharesToBlobs(rawShares [][]byte) ([]*Blob, error) {
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

func rowsToCids(rows [][]byte) []cid.Cid {
	cids := make([]cid.Cid, len(rows))
	for i, row := range rows {
		cids[i] = ipld.MustCidFromNamespacedSha256(row)
	}
	return cids
}
