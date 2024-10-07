package blob

import (
	"fmt"
	"sort"

	gosquare "github.com/celestiaorg/go-square/v2/share"
)

// BlobsToShares accepts blobs and convert them to the Shares.
func BlobsToShares(nodeBlobs ...*Blob) ([]gosquare.Share, error) {
	sort.Slice(nodeBlobs, func(i, j int) bool {
		return nodeBlobs[i].Blob.Namespace().Compare(nodeBlobs[j].Blob.Namespace()) < 0
	})

	splitter := gosquare.NewSparseShareSplitter()
	for i, nodeBlob := range nodeBlobs {
		err := splitter.Write(nodeBlob.Blob)
		if err != nil {
			return nil, fmt.Errorf("failed to split blob at index: %d: %w", i, err)
		}

	}
	return splitter.Export(), nil
}

// ToAppBlobs converts node's blob type to the blob type from go-square.
func ToAppBlobs(blobs ...*Blob) []*gosquare.Blob {
	appBlobs := make([]*gosquare.Blob, len(blobs))
	for i := range blobs {
		appBlobs[i] = blobs[i].Blob
	}
	return appBlobs
}

func calculateIndex(rowLength, blobIndex int) (row, col int) {
	row = blobIndex / rowLength
	col = blobIndex - (row * rowLength)
	return
}
