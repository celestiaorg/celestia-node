package blobtest

import (
	"fmt"

	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/v3/test/util/testfactory"
	"github.com/celestiaorg/go-square/shares"
	"github.com/celestiaorg/go-square/v2/share"
)

// GenerateV0Blobs is a test utility producing v0 share formatted blobs with the
// requested size and random namespaces.
func GenerateV0Blobs(sizes []int, sameNamespace bool) ([]*share.Blob, error) {
	blobs := make([]*share.Blob, 0, len(sizes))
	for _, size := range sizes {
		size := RawBlobSize(share.FirstSparseShareContentSize * size)
		appBlob := testfactory.GenerateRandomBlob(size)
		if !sameNamespace {
			namespace, err := share.NewV0Namespace(tmrand.Bytes(7))
			if err != nil {
				return nil, err
			}
			if namespace.IsReserved() {
				return nil, fmt.Errorf("reserved namespace")
			}
			appBlob, err = share.NewV0Blob(namespace, appBlob.Data())
			if err != nil {
				return nil, err
			}
		}

		blobs = append(blobs, appBlob)
	}
	return blobs, nil
}

func RawBlobSize(totalSize int) int {
	return totalSize - shares.DelimLen(uint64(totalSize))
}
