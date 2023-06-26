package blobtest

import (
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/test/util/testfactory"

	"github.com/celestiaorg/celestia-node/share"
)

// GenerateV0Blobs is a test utility producing v0 share formatted blobs with the
// requested size and random namespaces.
func GenerateV0Blobs(sizes []int, sameNamespace bool) ([]types.Blob, error) {
	blobs := make([]types.Blob, 0, len(sizes))

	for _, size := range sizes {
		size := rawBlobSize(appconsts.FirstSparseShareContentSize * size)
		appBlob := testfactory.GenerateRandomBlob(size)
		if !sameNamespace {
			nid, err := share.NewBlobNamespaceV0(tmrand.Bytes(7))
			if err != nil {
				return nil, err
			}
			appBlob.NamespaceVersion = nid[0]
			appBlob.NamespaceID = nid[1:]
		}

		blobs = append(blobs, appBlob)
	}
	return blobs, nil
}

func rawBlobSize(totalSize int) int {
	return totalSize - shares.DelimLen(uint64(totalSize))
}
