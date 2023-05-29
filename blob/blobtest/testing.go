package blobtest

import (
	"bytes"

	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/test/util/testfactory"
)

func GenerateBlobs(sizes []int, sameNID bool) ([]types.Blob, error) {
	nID := append(appns.NamespaceVersionZeroPrefix, bytes.Repeat([]byte{0x1}, appns.NamespaceVersionZeroIDSize)...)
	blobs := make([]types.Blob, 0, len(sizes))

	for _, size := range sizes {
		size := rawBlobSize(appconsts.FirstSparseShareContentSize * size)
		appBlob := testfactory.GenerateRandomBlob(size)
		if sameNID {
			appBlob.NamespaceID = nID
		}

		blobs = append(blobs, appBlob)
	}
	return blobs, nil
}

func rawBlobSize(totalSize int) int {
	return totalSize - shares.DelimLen(uint64(totalSize))
}
