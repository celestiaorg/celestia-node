package blob

import (
	"bytes"
	"errors"
	"sort"

	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/shares"

	"github.com/celestiaorg/celestia-node/share"
)

// parseShares takes shares and converts them to the *Blob.
func parseShares(appShrs []shares.Share, index int) (*Blob, error) {
	sequence, err := shares.ParseShares(appShrs, true)
	if err != nil {
		return nil, err
	}

	// ensure that sequence length is not 0
	if len(sequence) == 0 {
		return nil, ErrBlobNotFound
	}
	if len(sequence) > 1 {
		return nil, errors.New("unexpected amount of sequences")
	}

	data, err := sequence[0].RawData()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrBlobNotFound
	}

	shareVersion, err := sequence[0].Shares[0].Version()
	if err != nil {
		return nil, err
	}

	blob, err := NewBlob(shareVersion, sequence[0].Namespace.Bytes(), data)
	if err != nil {
		return nil, err
	}
	blob.index = index
	return blob, nil
}

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
