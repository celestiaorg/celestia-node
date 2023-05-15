package blob

import (
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
)

// sharesToBlobs takes raw shares and converts them to blobs.
func sharesToBlobs(rawShares [][]byte) ([]*Blob, error) {
	if len(rawShares) == 0 {
		return nil, ErrBlobNotFound
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

const (
	perByteGasTolerance = 2
	pfbGasFixedCost     = 80000
)

// estimateGas estimates the gas required to pay for a set of blobs in a PFB.
func estimateGas(blobs ...*Blob) uint64 {
	totalByteCount := 0
	for _, blob := range blobs {
		totalByteCount += len(blob.Data()) + appconsts.NamespaceSize
	}
	variableGasAmount := (appconsts.DefaultGasPerBlobByte + perByteGasTolerance) * totalByteCount

	return uint64(variableGasAmount + pfbGasFixedCost)
}
