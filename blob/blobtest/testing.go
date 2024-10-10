package blobtest

import (
	"encoding/binary"
	"fmt"

	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/v3/test/util/testfactory"
	gosquare "github.com/celestiaorg/go-square/v2/share"
)

// GenerateV0Blobs is a test utility producing v0 share formatted blobs with the
// requested size and random namespaces.
func GenerateV0Blobs(sizes []int, sameNamespace bool) ([]*gosquare.Blob, error) {
	blobs := make([]*gosquare.Blob, 0, len(sizes))
	for _, size := range sizes {
		size := RawBlobSize(gosquare.FirstSparseShareContentSize * size)
		appBlob := testfactory.GenerateRandomBlob(size)
		if !sameNamespace {
			namespace, err := gosquare.NewV0Namespace(tmrand.Bytes(7))
			if err != nil {
				return nil, err
			}
			if namespace.IsReserved() {
				return nil, fmt.Errorf("reserved namespace")
			}
			appBlob, err = gosquare.NewV0Blob(namespace, appBlob.Data())
			if err != nil {
				return nil, err
			}
		}

		blobs = append(blobs, appBlob)
	}
	return blobs, nil
}

func RawBlobSize(totalSize int) int {
	return totalSize - delimLen(uint64(totalSize))
}

// delimLen calculates the length of the delimiter for a given unit size
func delimLen(size uint64) int {
	lenBuf := make([]byte, binary.MaxVarintLen64)
	return binary.PutUvarint(lenBuf, size)
}
