package blob

import (
	"sort"

	squareblob "github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/shares"

	"github.com/celestiaorg/celestia-node/share"
)

// BlobsToShares accepts blobs and convert them to the Shares.
func BlobsToShares(nodeBlobs ...*Blob) ([]share.Share, error) {
	b := make([]*squareblob.Blob, len(nodeBlobs))
	for i, nodeBlob := range nodeBlobs {
		namespace := nodeBlob.Namespace()
		b[i] = &squareblob.Blob{
			NamespaceVersion: uint32(namespace[0]),
			NamespaceId:      namespace[1:],
			Data:             nodeBlob.Data,
			ShareVersion:     nodeBlob.ShareVersion,
		}
	}

	sort.Slice(b, func(i, j int) bool {
		return b[i].Namespace().Compare(b[j].Namespace()) < 0
	})

	rawShares, err := shares.SplitBlobs(b...)
	if err != nil {
		return nil, err
	}
	return shares.ToBytes(rawShares), nil
}

// ToAppBlobs converts node's blob type to the blob type from go-square.
func ToAppBlobs(blobs ...*Blob) []*squareblob.Blob {
	appBlobs := make([]*squareblob.Blob, len(blobs))
	for i := range blobs {
		appBlobs[i] = blobs[i].Blob
	}
	return appBlobs
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
