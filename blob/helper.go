package blob

import (
	"bytes"
	"sort"

	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"

	"github.com/celestiaorg/celestia-node/share"
)

// SharesToBlobs takes raw shares and converts them to the blobs.
func SharesToBlobs(rawShares []share.Share) ([]*Blob, error) {
	if len(rawShares) == 0 {
		return nil, ErrBlobNotFound
	}

	appShares := make([]shares.Share, 0, len(rawShares))
	for _, shr := range rawShares {
		bShare, err := shares.NewShare(shr)
		if err != nil {
			return nil, err
		}
		appShares = append(appShares, *bShare)
	}

	shareSequences, err := shares.ParseShares(appShares, true)
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

		shareVersion, err := sequence.Shares[0].Version()
		if err != nil {
			return nil, err
		}

		blob, err := NewBlob(shareVersion, sequence.Namespace.Bytes(), data)
		if err != nil {
			return nil, err
		}
		blobs[i] = blob
	}
	return blobs, nil
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
		return val <= 0
	})

	rawShares, err := shares.SplitBlobs(b...)
	if err != nil {
		return nil, err
	}
	return shares.ToBytes(rawShares), nil
}

const (
	perByteGasTolerance = 2
	pfbGasFixedCost     = 80000
)

// estimateGas estimates the gas required to pay for a set of blobs in a PFB.
func estimateGas(blobs ...*Blob) uint64 {
	totalByteCount := 0
	for _, blob := range blobs {
		totalByteCount += len(blob.Data) + appconsts.NamespaceSize
	}
	variableGasAmount := (appconsts.DefaultGasPerBlobByte + perByteGasTolerance) * totalByteCount

	return uint64(variableGasAmount + pfbGasFixedCost)
}

// constructAndVerifyBlob reconstruct a Blob from the passed shares and compares commitments.
func constructAndVerifyBlob(sh []share.Share, commitment Commitment) (*Blob, bool, error) {
	blob, err := SharesToBlobs(sh)
	if err != nil {
		return nil, false, err
	}

	equal := blob[0].Commitment.Equal(commitment)
	return blob[0], equal, nil
}
