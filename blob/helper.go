package blob

import (
	"sort"

	"github.com/celestiaorg/go-square/merkle"
	"github.com/celestiaorg/go-square/v3/inclusion"
	libshare "github.com/celestiaorg/go-square/v3/share"
)

// BlobsToShares accepts blobs and convert them to the Shares.
func BlobsToShares(nodeBlobs ...*Blob) ([]libshare.Share, error) {
	sort.Slice(nodeBlobs, func(i, j int) bool {
		return nodeBlobs[i].Blob.Namespace().IsLessThan(nodeBlobs[j].Blob.Namespace())
	})

	shares := make([]libshare.Share, 0)
	for _, nodeBlob := range nodeBlobs {
		sh, err := nodeBlob.ToShares()
		if err != nil {
			return nil, err
		}
		shares = append(shares, sh...)
	}
	return shares, nil
}

// ToLibBlobs converts node's blob type to the blob type from go-square.
func ToLibBlobs(blobs ...*Blob) []*libshare.Blob {
	libBlobs := make([]*libshare.Blob, len(blobs))
	for i := range blobs {
		libBlobs[i] = blobs[i].Blob
	}
	return libBlobs
}

// ToNodeBlobs converts libshare blob type to the node's specific blob type.
func ToNodeBlobs(blobs ...*libshare.Blob) ([]*Blob, error) {
	nodeBlobs := make([]*Blob, len(blobs))
	hashFromByteSlices := merkle.HashFromByteSlices
	for i, blob := range blobs {
		com, err := inclusion.CreateCommitment(blob, hashFromByteSlices, subtreeRootThreshold)
		if err != nil {
			return nil, err
		}
		nodeBlobs[i] = &Blob{Blob: blob, Commitment: com, index: -1}
	}
	return nodeBlobs, nil
}

func calculateIndex(rowLength, blobIndex int) (row, col int) {
	row = blobIndex / rowLength
	col = blobIndex - (row * rowLength)
	return row, col
}
