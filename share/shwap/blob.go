package shwap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/celestiaorg/celestia-app/v7/pkg/appconsts"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/go-square/merkle"
	"github.com/celestiaorg/go-square/v3/inclusion"
	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

const blobName = "blob_v0"

var subtreeRootThreshold = appconsts.SubtreeRootThreshold

var ErrBlobNotFound = errors.New("blob: not found")

// Blob represents a retrieved blob from the data square containing
// the blob data along with proofs for the verification and blob's index(position inside the ODS).
type Blob struct {
	// RangeNamespaceData contains the blob shares and cryptographic proofs
	// for verifying the blob's inclusion in the Extended Data Square.
	// This includes namespace proofs, row proofs, and the actual share data.
	*RangeNamespaceData

	// StartIndex is the position of the blob inside the data square(*ODS*).
	StartIndex int
}

// BlobsFromShares reconstructs Blobs from a slice of row shares by finding blobs
// that match the given namespace within the provided shares.
//
// When commitments are provided, only blobs matching those commitments are returned.
// When no commitments are provided, all blobs in the namespace are returned.
func BlobsFromShares(
	libShares [][]libshare.Share,
	namespace libshare.Namespace,
	odsSize int,
	commitments ...[]byte,
) ([]*Blob, error) {
	if libShares == nil || libShares[0] == nil {
		return nil, errors.New("empty share list")
	}

	filtering := len(commitments) > 0
	var blobs []*Blob
	colStart := 0
	for rowStart := 0; rowStart < len(libShares); {
		shr := libShares[rowStart][colStart]
		if !shr.Namespace().Equals(namespace) || !shr.IsSequenceStart() || shr.IsPadding() {
			colStart++
			if colStart >= odsSize {
				colStart = 0
				rowStart++
			}
			continue
		}

		sharesAmount := libshare.SparseSharesNeeded(shr.SequenceLen(), shr.ContainsSigner())

		from := SampleCoords{Row: rowStart, Col: colStart}
		fromIndex, err := SampleCoordsAs1DIndex(from, odsSize)
		if err != nil {
			return nil, err
		}
		endIndex := fromIndex + sharesAmount - 1
		to, err := SampleCoordsFrom1DIndex(endIndex, odsSize)
		if err != nil {
			return nil, err
		}

		if filtering {
			// compute commitment to check against requested commitments
			rngShares := libShares[rowStart : to.Row+1]
			shrs := make([]libshare.Share, 0, sharesAmount)
			for i := range rngShares {
				startCol := 0
				endCol := odsSize
				if i == 0 {
					startCol = from.Col
				}
				if i == len(rngShares)-1 {
					endCol = to.Col + 1
				}
				shrs = append(shrs, rngShares[i][startCol:endCol]...)
			}

			parsed, err := libshare.ParseBlobs(shrs)
			if err != nil {
				return nil, err
			}
			if len(parsed) != 1 {
				return nil, fmt.Errorf("expected exactly one blob, got %d", len(parsed))
			}
			com, err := inclusion.CreateCommitment(parsed[0], merkle.HashFromByteSlices, subtreeRootThreshold)
			if err != nil {
				return nil, err
			}

			if !matchesCommitment(com, commitments) {
				// advance past the current blob
				nextIndex := endIndex + 1
				if nextIndex >= odsSize*odsSize {
					break
				}
				coords, err := SampleCoordsFrom1DIndex(nextIndex, odsSize)
				if err != nil {
					return nil, err
				}
				rowStart = coords.Row
				colStart = coords.Col
				continue
			}
		}

		rngData, err := RangeNamespaceDataFromShares(libShares[rowStart:to.Row+1], from, to)
		if err != nil {
			return nil, err
		}

		blobs = append(blobs, &Blob{RangeNamespaceData: &rngData, StartIndex: fromIndex})

		// advance past the current blob
		nextIndex := endIndex + 1
		if nextIndex >= odsSize*odsSize {
			break
		}
		coords, err := SampleCoordsFrom1DIndex(nextIndex, odsSize)
		if err != nil {
			return nil, err
		}
		rowStart = coords.Row
		colStart = coords.Col
	}

	if len(blobs) == 0 {
		if filtering {
			return nil, ErrBlobNotFound
		}
		return nil, ErrNotFound
	}
	return blobs, nil
}

func matchesCommitment(com []byte, commitments [][]byte) bool {
	for _, c := range commitments {
		if bytes.Equal(com, c) {
			return true
		}
	}
	return false
}

func (b *Blob) VerifyInclusion(roots [][]byte) error {
	length := libshare.SparseSharesNeeded(b.Shares[0][0].SequenceLen(), b.Shares[0][0].ContainsSigner())
	odsSize := len(roots) / 2

	from, err := SampleCoordsFrom1DIndex(b.StartIndex, odsSize)
	if err != nil {
		return err
	}
	to, err := SampleCoordsFrom1DIndex(b.StartIndex+length-1, odsSize)
	if err != nil {
		return err
	}
	return b.RangeNamespaceData.VerifyInclusion(from, to, odsSize, roots[from.Row:to.Row+1])
}

func (b *Blob) Verify(roots [][]byte, commitment []byte) error {
	if len(b.Shares) == 0 || len(b.Shares[0]) == 0 {
		return errors.New("blob is empty")
	}
	length := libshare.SparseSharesNeeded(b.Shares[0][0].SequenceLen(), b.Shares[0][0].ContainsSigner())
	flatten := b.Flatten()
	if length != len(flatten) {
		return fmt.Errorf("mismatched blob length expected: %d, got: %d", length, len(flatten))
	}
	blobs, err := libshare.ParseBlobs(flatten)
	if err != nil {
		return err
	}
	if len(blobs) != 1 {
		return fmt.Errorf("expected exactly one blob, got %d", len(blobs))
	}

	com, err := inclusion.CreateCommitment(blobs[0], merkle.HashFromByteSlices, subtreeRootThreshold)
	if err != nil {
		return err
	}
	if !bytes.Equal(com, commitment) {
		return errors.New("commitments mismatch")
	}
	return b.VerifyInclusion(roots)
}

// Index an ODS index of the retrieved blob
func (b *Blob) Index() int {
	return b.StartIndex
}

// Blob creates a blob from the set of shares that are part of the blob container
func (b *Blob) Blob() (*libshare.Blob, error) {
	if b.Shares == nil || b.Shares[0] == nil {
		return nil, errors.New("blob is empty")
	}

	length := libshare.SparseSharesNeeded(b.Shares[0][0].SequenceLen(), b.Shares[0][0].ContainsSigner())
	shrs := make([]libshare.Share, 0, length)
	for i := range b.Shares {
		shrs = append(shrs, b.Shares[i]...)
	}
	blob, err := libshare.ParseBlobs(shrs)
	if err != nil {
		return nil, err
	}

	return blob[0], nil
}

func (b *Blob) Commitment() ([]byte, error) {
	blob, err := b.Blob()
	if err != nil {
		return nil, err
	}
	return inclusion.CreateCommitment(blob, merkle.HashFromByteSlices, subtreeRootThreshold)
}

func (b *Blob) Length() (int, error) {
	if b == nil || b.IsEmpty() {
		return 0, errors.New("blob is empty")
	}
	return libshare.SparseSharesNeeded(b.Shares[0][0].SequenceLen(), b.Shares[0][0].ContainsSigner()), nil
}

func (b *Blob) ToProto() *pb.Blob {
	return &pb.Blob{
		RngData:    b.RangeNamespaceData.ToProto(),
		StartIndex: uint64(b.StartIndex),
	}
}

func BlobFromProto(pbBlob *pb.Blob) (Blob, error) {
	if pbBlob.StartIndex > math.MaxInt {
		return Blob{}, fmt.Errorf("start index overflows int: %d", pbBlob.StartIndex)
	}
	rngData, err := RangeNamespaceDataFromProto(pbBlob.RngData)
	if err != nil {
		return Blob{}, err
	}
	return Blob{
		RangeNamespaceData: &rngData,
		StartIndex:         int(pbBlob.StartIndex),
	}, nil
}

func (b *Blob) WriteTo(w io.Writer) (int64, error) {
	pbBlob := b.ToProto()
	n, err := serde.Write(w, pbBlob)
	return int64(n), err
}

func (b *Blob) ReadFrom(r io.Reader) (int64, error) {
	pbBlob := &pb.Blob{}
	n, err := serde.Read(r, pbBlob)
	if err != nil {
		return 0, err
	}

	*b, err = BlobFromProto(pbBlob)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

type BlobSlice []*Blob

func (bs *BlobSlice) ReadFrom(reader io.Reader) (int64, error) {
	var blobNew []*Blob
	var totalRead int64

	for {
		var blob Blob
		nn, err := blob.ReadFrom(reader)
		totalRead += nn
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return totalRead, err
		}

		blobNew = append(blobNew, &blob)
	}

	*bs = blobNew
	return totalRead, nil
}

func (bs *BlobSlice) WriteTo(w io.Writer) (int64, error) {
	var totalWritten int64
	for _, blob := range *bs {
		n, err := blob.WriteTo(w)
		totalWritten += n
		if err != nil {
			return n, err
		}
	}
	return totalWritten, nil
}
