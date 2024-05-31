package blob

import (
	"bytes"
	"sort"

	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/shares"

	"github.com/celestiaorg/celestia-node/share"
)

// BlobsToShares accepts blobs and convert them to the Shares.
func BlobsToShares(nodeBlobs ...*Blob) ([]share.Share, error) {
	b := make([]*blob.Blob, len(nodeBlobs))
	for i, nodeBlob := range nodeBlobs {
		namespace := nodeBlob.Namespace()
		b[i] = &blob.Blob{
			NamespaceVersion: uint32(namespace[0]),
			NamespaceId:      namespace[1:],
			Data:             nodeBlob.Data,
			ShareVersion:     uint32(nodeBlob.ShareVersion),
		}
	}

	sort.Slice(b, func(i, j int) bool {
		val := bytes.Compare(b[i].Namespace().Bytes(), b[j].Namespace().Bytes())
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
