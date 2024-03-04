package blob

import (
	"bytes"
	"sort"

	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/shares"

	"github.com/celestiaorg/celestia-node/share"
)

// BlobsToShares accepts blobs and convert them to the Shares.
func BlobsToShares(blobs ...*Blob) ([]share.Share, error) {
	b := make([]types.Blob, len(blobs))
	for i, blob := range blobs {
		namespace := blob.Namespace()
		b[i] = types.Blob{
			NamespaceVersion: namespace[0],
			NamespaceID:      namespace[1:],
			Data:             blob.Data,
			ShareVersion:     uint8(blob.ShareVersion),
		}
	}

	sort.Slice(b, func(i, j int) bool {
		val := bytes.Compare(b[i].NamespaceID, b[j].NamespaceID)
		return val < 0
	})

	rawShares, err := shares.SplitBlobs(b...)
	if err != nil {
		return nil, err
	}
	return shares.ToBytes(rawShares), nil
}

// toAppShares converts node's raw shares to the app shares, skipping padding
func toAppShares(shrs ...share.Share) ([]shares.Share, error) {
	appShrs := make([]shares.Share, 0, len(shrs))
	for _, shr := range shrs {
		bShare, err := shares.NewShare(shr)
		if err != nil {
			return nil, err
		}
		appShrs = append(appShrs, *bShare)
	}
	return appShrs, nil
}

func calculateIndex(rowLength, blobIndex int) (row, col int) {
	row = blobIndex / rowLength
	col = blobIndex - (row * rowLength)
	return
}
