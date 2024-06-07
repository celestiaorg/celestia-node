package blobtest

import (
	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v2/test/util/testfactory"
	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/shares"

	"github.com/celestiaorg/celestia-node/share"
)

// GenerateV0Blobs is a test utility producing v0 share formatted blobs with the
// requested size and random namespaces.
func GenerateV0Blobs(sizes []int, sameNamespace bool) ([]*blob.Blob, error) {
	blobs := make([]*blob.Blob, 0, len(sizes))

	for _, size := range sizes {
		size := rawBlobSize(appconsts.FirstSparseShareContentSize * size)
		appBlob := testfactory.GenerateRandomBlob(size)
		if !sameNamespace {
			namespace, err := share.NewBlobNamespaceV0(tmrand.Bytes(7))
			if err != nil {
				return nil, err
			}
			appBlob.NamespaceVersion = uint32(namespace[0])
			appBlob.NamespaceId = namespace[1:]
		}

		blobs = append(blobs, appBlob)
	}
	return blobs, nil
}

func rawBlobSize(totalSize int) int {
	return totalSize - shares.DelimLen(uint64(totalSize))
}
