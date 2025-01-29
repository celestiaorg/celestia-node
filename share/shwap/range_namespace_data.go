package shwap

import (
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto/merkle"
	corebytes "github.com/tendermint/tendermint/libs/bytes"
	coretypes "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/v3/pkg/wrapper"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
	libshare "github.com/celestiaorg/go-square/v2/share"
)

// RangeNamespaceData embeds `NamespaceData` and contains a contiguous range of shares
// along with proofs for these shares.
type RangeNamespaceData struct {
	NamespaceData
}

// RangedNamespaceDataFromShares builds a range of namespaced data for the given coordinates:
// shares is a list of shares (grouped by the rows) relative to the data square and
// needed to build the range;
// namespace is the target namespace for the built range;
// from is the coordinates of the first share of the range within the EDS.
// to is the coordinates of the last inclusive share of the range within the EDS.
func RangedNamespaceDataFromShares(
	shares [][]libshare.Share,
	namespace libshare.Namespace,
	from, to SampleCoords,
) (RangeNamespaceData, error) {
	if len(shares) == 0 {
		return RangeNamespaceData{}, fmt.Errorf("empty share list")
	}

	odsSize := len(shares[0]) / 2
	nsData := make([]RowNamespaceData, 0, len(shares))
	for i, row := 0, from.Row; i < len(shares); i++ {
		rowShares := shares[i]
		// end index will be explicitly set only for the last row in range.
		// in other cases, it will be equal to the odsSize.
		exclusiveEnd := odsSize
		if i == len(shares)-1 {
			// `to.Col` is an inclusive index
			exclusiveEnd = to.Col + 1
		}

		// ensure that all shares from the range belong to the requested namespace
		for offset := range rowShares[from.Col:exclusiveEnd] {
			if !namespace.Equals(rowShares[from.Col+offset].Namespace()) {
				return RangeNamespaceData{},
					fmt.Errorf("targeted namespace was not found in share at {Row: %d, Col: %d}",
						row, from.Col+offset,
					)
			}
		}

		tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(odsSize), uint(row))

		for _, shr := range rowShares {
			if err := tree.Push(shr.ToBytes()); err != nil {
				return RangeNamespaceData{}, fmt.Errorf("failed to build tree for row %d: %w", row, err)
			}
		}

		root, err := tree.Root()
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to get root for row %d: %w", row, err)
		}

		outside, err := share.IsOutsideRange(namespace, root, root)
		if err != nil {
			return RangeNamespaceData{}, err
		}
		if outside {
			return RangeNamespaceData{}, ErrNamespaceOutsideRange
		}

		proof, err := tree.ProveRange(from.Col, exclusiveEnd)
		if err != nil {
			return RangeNamespaceData{}, err
		}

		nsData = append(nsData, RowNamespaceData{Shares: rowShares[from.Col:exclusiveEnd], Proof: &proof})

		// reset from.Col as we are moving to the next row.
		from.Col = 0
		row++
	}
	return RangeNamespaceData{nsData}, nil
}

// Verify performs a validation of the incoming data. It ensures that the response contains proofs and performs
// data validation in case the user has requested it.
func (rngdata *RangeNamespaceData) Verify(root *share.AxisRoots, req *RangeNamespaceDataID) error {
	_, err := rngdata.IsEmpty()
	if err != nil {
		return fmt.Errorf("RangeNamespaceData: empty data: %w", err)
	}

	if rngdata.NamespaceData[0].Proof.Start() != req.ShareIndex {
		return fmt.Errorf("RangeNamespaceData: invalid start of the range: want: %d, got: %d",
			req.ShareIndex, rngdata.NamespaceData[0].Proof.Start(),
		)
	}

	// verify shares amount
	rawShares := rngdata.Flatten()
	odsFromIdx, err := SampleCoordsAs1DIndex(
		SampleCoords{req.RowIndex, req.ShareIndex}, len(root.RowRoots)/2,
	)
	if err != nil {
		return err
	}

	odsToIdx, err := SampleCoordsAs1DIndex(req.To, len(root.RowRoots)/2)
	if err != nil {
		return err
	}
	if len(rawShares) != (odsToIdx - odsFromIdx + 1) { // `+1` because both indexes are inclusive
		return fmt.Errorf("RangeNamespaceData: shares amount mismatch: want: %d, got: %d",
			odsToIdx-odsFromIdx+1, len(rawShares),
		)
	}

	rowStart := req.RowIndex
	for i, row := range rngdata.NamespaceData {
		verified := row.Proof.VerifyInclusion(
			share.NewSHA256Hasher(),
			req.DataNamespace.Bytes(),
			libshare.ToBytes(row.Shares),
			root.RowRoots[rowStart+i],
		)
		if !verified {
			return fmt.Errorf("RangeNamespaceData: %w at row: %d", ErrFailedVerification, rowStart+i)
		}
	}
	return nil
}

// IsEmpty verifies whether the underlying `NamespaceData` is empty.
// It returns an error in case the `RangeNamespaceData` is completely empty
func (rngdata RangeNamespaceData) IsEmpty() (bool, error) {
	if len(rngdata.NamespaceData) == 0 {
		return true, errors.New("namespace data is empty")
	}

	for _, row := range rngdata.NamespaceData {
		if row.Proof == nil {
			return true, errors.New("proof list for the row is empty")
		}
		if row.IsEmpty() {
			return true, nil
		}
	}
	return false, nil
}

func (rngdata *RangeNamespaceData) ToProto() *pb.RangeNamespaceData {
	return &pb.RangeNamespaceData{RangeNamespaceData: rngdata.NamespaceData.ToProto()}
}

func RangeNamespaceDataFromProto(nd *pb.RangeNamespaceData) (*RangeNamespaceData, error) {
	if nd == nil {
		return nil, errors.New("empty data provided")
	}
	nsRowData, err := NamespaceDataFromProto(nd.RangeNamespaceData)
	if err != nil {
		return nil, fmt.Errorf("failed to build namespace data from proto: %w", err)
	}
	return &RangeNamespaceData{NamespaceData: nsRowData}, nil
}

// ProveRange proves that a range of shares exist in the set of rows that are part of the merkle tree of the data root.
func (rngdata *RangeNamespaceData) ProveRange(roots *share.AxisRoots, startRow int) *types.ShareProof {
	// 1. convert nmt.Proof to the core type of NmtProof
	nmtProofs := make([]*coretypes.NMTProof, len(rngdata.NamespaceData))
	for i, row := range rngdata.NamespaceData {
		nmtProofs[i] = &coretypes.NMTProof{
			Start:    int32(row.Proof.Start()),
			End:      int32(row.Proof.End()),
			Nodes:    row.Proof.Nodes(),
			LeafHash: row.Proof.LeafHash(),
		}
	}

	// 2. create the merkle inclusion proof for all rows to the data root
	_, proofs := merkle.ProofsFromByteSlices(append(roots.RowRoots, roots.ColumnRoots...))

	endRowExclusive := startRow + len(rngdata.NamespaceData)
	// 3. pick needed rowRoots along with their proofs
	rowProofs := proofs[startRow:endRowExclusive]
	rowRoots := make([]corebytes.HexBytes, endRowExclusive-startRow)
	for i, rowRoot := range roots.RowRoots[startRow:endRowExclusive] {
		rowRoots[i] = rowRoot
	}
	return &types.ShareProof{
		Data:        libshare.ToBytes(rngdata.Flatten()),
		ShareProofs: nmtProofs,
		NamespaceID: rngdata.NamespaceData[0].Shares[0].Namespace().ID(),
		RowProof: types.RowProof{
			RowRoots: rowRoots,
			Proofs:   rowProofs,
			StartRow: uint32(startRow),
			EndRow:   uint32(endRowExclusive) - 1,
		},
		NamespaceVersion: uint32(rngdata.NamespaceData[0].Shares[0].Namespace().Version()),
	}
}

// RangeCoordsFromIdx accepts the start index and the length of the range and
// computes `shwap.SampleCoords` of the first and the last sample of this range.
// * edsIndex is the index of the first sample inside the eds;
// * length is the amount of *ORIGINAL* samples that are expected to get(including the first sample);
func RangeCoordsFromIdx(edsIndex, length, edsSize int) (SampleCoords, SampleCoords, error) {
	from, err := SampleCoordsFrom1DIndex(edsIndex, edsSize)
	if err != nil {
		return SampleCoords{}, SampleCoords{}, err
	}
	odsIndex, err := SampleCoordsAs1DIndex(from, edsSize/2)
	if err != nil {
		return SampleCoords{}, SampleCoords{}, err
	}

	toInclusive := odsIndex + length - 1
	toCoords, err := SampleCoordsFrom1DIndex(toInclusive, edsSize/2)
	if err != nil {
		return SampleCoords{}, SampleCoords{}, err
	}
	return from, toCoords, nil
}
