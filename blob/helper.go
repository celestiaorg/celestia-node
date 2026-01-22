package blob

import (
	"fmt"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
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

func fromShwapBlob(shBlob *shwap.Blob, odsSize int) (*Blob, error) {
	blb, err := shBlob.Blob()
	if err != nil {
		return nil, err
	}

	commitment, err := shBlob.Commitment()
	if err != nil {
		return nil, err
	}

	odsIndex := shBlob.Index()
	coords, err := shwap.SampleCoordsFrom1DIndex(odsIndex, odsSize)
	if err != nil {
		return nil, err
	}

	edsIndex, err := shwap.SampleCoordsAs1DIndex(coords, odsIndex*2)
	if err != nil {
		return nil, err
	}
	return &Blob{
		Blob:       blb,
		Commitment: commitment,
		index:      edsIndex,
	}, nil
}

func buildProof(blob *shwap.Blob, odsSize int) (*Proof, error) {
	shares := blob.Shares
	numRows := len(shares)
	coords, err := shwap.SampleCoordsFrom1DIndex(blob.Index(), odsSize)
	if err != nil {
		return nil, err
	}
	proofs := make(Proof, numRows)
	if blob.FirstIncompleteRowProof != nil {
		proofs[0] = blob.FirstIncompleteRowProof
	}
	if blob.LastIncompleteRowProof != nil {
		proofs[numRows-1] = blob.LastIncompleteRowProof
	}

	for i := 0; i < numRows; i++ {
		if proofs[i] != nil {
			continue
		}
		if len(shares[i]) != odsSize {
			return nil, fmt.Errorf("incomplete row of shares at: %d, %d != %d",
				coords.Row+i, len(shares[i]), odsSize,
			)
		}

		extendedShares, err := share.ExtendShares(shares[i])
		if err != nil {
			return nil, err
		}

		proof, err := shwap.GenerateSharesProofs(
			coords.Row+i,
			0,
			odsSize,
			odsSize,
			extendedShares,
		)
		if err != nil {
			return nil, err
		}
		proofs[i] = proof
	}
	return &proofs, nil
}
