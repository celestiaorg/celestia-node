package shwap

import (
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/crypto/merkle"

	"github.com/celestiaorg/celestia-app/v4/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// RangeNamespaceData embeds `NamespaceData` and contains a contiguous range of shares
// along with proofs for these shares.
type RangeNamespaceData struct {
	Start  int                `json:"start"`
	Shares [][]libshare.Share `json:"shares,omitempty"`
	Proof  []*Proof           `json:"proof"`
}

// RangedNamespaceDataFromShares constructs a RangeNamespaceData structure from a selection
// of namespaced shares within an Extended Data Square (EDS).
//
// Parameters:
//   - shares: A 2D slice of shares grouped by rows (each inner slice represents a row).
//     These shares should cover the range between the 'from' and 'to' coordinates, inclusive.
//   - namespace: The target namespace ID to filter and extract data for.
//   - axisRoots: The AxisRoots (row and column roots) associated with the EDS.
//   - from: The starting coordinate (row, column) of the range within the EDS.
//   - to: The ending coordinate (row, column) of the range (inclusive).
//
// Returns:
//   - A RangeNamespaceData containing the shares in the given namespace and Merkle proofs
//     needed to verify them.
//   - An error if the range is invalid or extraction fails.
func RangedNamespaceDataFromShares(
	shares [][]libshare.Share,
	namespace libshare.Namespace,
	axisRoots *share.AxisRoots,
	from, to SampleCoords,
) (RangeNamespaceData, error) {
	if len(shares) == 0 {
		return RangeNamespaceData{}, fmt.Errorf("empty share list")
	}

	roots := append(axisRoots.RowRoots, axisRoots.ColumnRoots...) //nolint: gocritic
	_, rowRootProofs := merkle.ProofsFromByteSlices(roots)

	numRows := len(shares)
	odsSize := len(shares[0]) / 2

	isMultiRow := from.Row != to.Row
	startsMidRow := from.Col != 0
	endsMidRow := to.Col != odsSize-1
	isSingleRow := from.Row == to.Row

	// We need a start proof if:
	// - The range doesn't start at the beginning of the row
	// - OR it's a single-row range that ends before the row ends
	startProof := startsMidRow || (isSingleRow && endsMidRow)

	// We need an end proof if:
	// - The range ends before the row ends
	// - AND spans multiple rows
	endProof := endsMidRow && isMultiRow

	rngData := RangeNamespaceData{
		Start:  from.Row,
		Shares: make([][]libshare.Share, numRows),
		Proof:  make([]*Proof, numRows),
	}
	for i, row, col := 0, from.Row, from.Col; i < numRows; i++ {
		rowShares := shares[i]
		// end index will be explicitly set only for the last row in range.
		// in other cases, it will be equal to the odsSize.
		exclusiveEnd := odsSize
		if i == len(shares)-1 {
			// `to.Col` is an inclusive index
			exclusiveEnd = to.Col + 1
		}

		// ensure that all shares from the range belong to the requested namespace
		for offset := range rowShares[col:exclusiveEnd] {
			if !namespace.Equals(rowShares[from.Col+offset].Namespace()) {
				return RangeNamespaceData{},
					fmt.Errorf("targeted namespace was not found in share at {Row: %d, Col: %d}",
						row, col+offset,
					)
			}
		}

		rngData.Shares[i] = rowShares[col:exclusiveEnd]
		rngData.Proof[i] = &Proof{
			rowRootProof: rowRootProofs[row],
			shareProof:   &nmt.Proof{},
		}

		// reset from.Col as we are moving to the next row.
		col = 0
		row++
	}
	// incomplete from.Col needs a proof for the first row to be computed
	if startProof {
		endCol := odsSize
		if isSingleRow {
			endCol = to.Col + 1
		}
		sharesProofs, err := generateSharesProofs(from.Row, from.Col, endCol, odsSize, namespace, shares[0])
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", from.Row, err)
		}
		rngData.Proof[0].shareProof = sharesProofs
	}
	// incomplete to.Col needs a proof for the last row to be computed
	if endProof {
		sharesProofs, err := generateSharesProofs(to.Row, 0, to.Col+1, odsSize, namespace, shares[numRows-1])
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", to.Row, err)
		}
		rngData.Proof[numRows-1].shareProof = sharesProofs
	}
	return rngData, nil
}

// Verify verifies the underlying shares are included in the data root.
func (rngdata *RangeNamespaceData) Verify(
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	dataHash []byte,
) error {
	return rngdata.VerifyShares(rngdata.Shares, namespace, from, to, dataHash)
}

// VerifyShares verifies the passed shares are included in the data root.
// `[][]libshare.Share` is a collection of the row shares from the range
// NOTE: the underlying shares will be ignored even if they are not empty.
func (rngdata *RangeNamespaceData) VerifyShares(
	shares [][]libshare.Share,
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	dataHash []byte,
) error {
	if from.Row != rngdata.Start {
		return fmt.Errorf("mismatched row: wanted: %d, got: %d", rngdata.Start, from.Row)
	}
	proofs := rngdata.Proof
	if len(shares) != len(proofs) {
		return fmt.Errorf("shares and proofs length mismatch")
	}
	// compute the proofs for the complete rows
	for i, row := 0, from.Row; i < len(shares); i++ {
		// if any of the row root proofs were not set, we cannot proceed
		if proofs[i].rowRootProof == nil {
			return fmt.Errorf("row root proof is empty for row: %d", rngdata.Start+i)
		}
		// skip the incomplete rows
		if !proofs[i].shareProof.IsEmptyProof() {
			continue
		}

		extendedRowShares, err := share.ExtendShares(shares[i])
		if err != nil {
			return fmt.Errorf("failed to extend row shares: %w", err)
		}

		rowNamespaceData, err := RowNamespaceDataFromShares(extendedRowShares, namespace, row)
		if err != nil {
			return fmt.Errorf("failed to get row namespace data: %w", err)
		}

		proofs[i].shareProof = rowNamespaceData.Proof
		row++
	}
	if len(shares) != len(proofs) {
		return fmt.Errorf(
			"mismatch amount of row shares and proofs, %d:%d",
			len(shares), len(rngdata.Proof),
		)
	}
	if rngdata.IsEmpty() {
		return errors.New("empty data")
	}
	if from.Col != proofs[0].Start() {
		return fmt.Errorf("mismatched col: wanted: %d, got: %d", proofs[0].Start(), from.Col)
	}
	if to.Col != proofs[len(proofs)-1].End() {
		return fmt.Errorf(
			"mismatched col: wanted: %d, got: %d",
			proofs[len(proofs)-1].End(), to.Col,
		)
	}

	for i, rowShares := range shares {
		if proofs[i].IsEmptyProof() {
			return fmt.Errorf("nil proof for row: %d", rngdata.Start+i)
		}
		if proofs[i].shareProof.IsOfAbsence() {
			return fmt.Errorf("absence proof for row: %d", rngdata.Start+i)
		}

		err := proofs[i].VerifyInclusion(rowShares, namespace, dataHash)
		if err != nil {
			return fmt.Errorf("%w for row: %d, %w", ErrFailedVerification, rngdata.Start+i, err)
		}
	}
	return nil
}

// Flatten combines all shares from all rows within the namespace into a single slice.
func (rngdata *RangeNamespaceData) Flatten() []libshare.Share {
	var shares []libshare.Share
	for _, shrs := range rngdata.Shares {
		shares = append(shares, shrs...)
	}
	return shares
}

func (rngdata *RangeNamespaceData) IsEmpty() bool {
	return len(rngdata.Shares) == 0 && len(rngdata.Proof) == 0
}

func (rngdata *RangeNamespaceData) ToProto() *pb.RangeNamespaceData {
	pbShares := make([]*pb.RowShares, len(rngdata.Shares))
	pbProofs := make([]*pb.Proof, len(rngdata.Proof))
	for i, shr := range rngdata.Shares {
		pbShares[i] = &pb.RowShares{}
		rowShares := SharesToProto(shr)
		pbShares[i].Shares = rowShares
	}

	for i, proof := range rngdata.Proof {
		pbProofs[i] = proof.ToProto()
	}

	return &pb.RangeNamespaceData{
		Start:  int32(rngdata.Start),
		Shares: pbShares,
		Proofs: pbProofs,
	}
}

func (rngdata *RangeNamespaceData) CleanupData() {
	rngdata.Shares = nil
}

func RangeNamespaceDataFromProto(nd *pb.RangeNamespaceData) (*RangeNamespaceData, error) {
	shares := make([][]libshare.Share, len(nd.Shares))

	for i, shr := range nd.Shares {
		shrs, err := SharesFromProto(shr.GetShares())
		if err != nil {
			return nil, err
		}
		shares[i] = shrs
	}

	proofs := make([]*Proof, len(nd.Proofs))
	for i, proof := range nd.Proofs {
		p, err := ProofFromProto(proof)
		if err != nil {
			return nil, err
		}
		proofs[i] = p
	}
	return &RangeNamespaceData{Start: int(nd.Start), Shares: shares, Proof: proofs}, nil
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

func generateSharesProofs(
	row, fromCol, toCol, size int,
	namespace libshare.Namespace,
	rowShares []libshare.Share,
) (*nmt.Proof, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size), uint(row))

	for i, shr := range rowShares {
		if err := tree.Push(shr.ToBytes()); err != nil {
			return nil, fmt.Errorf("failed to build tree at share index %d (row %d): %w", i, row, err)
		}
	}

	root, err := tree.Root()
	if err != nil {
		return nil, fmt.Errorf("failed to get root for row %d: %w", row, err)
	}

	// Check if the namespace is actually present in the row's range.
	outside, err := share.IsOutsideRange(namespace, root, root)
	if err != nil {
		return nil, fmt.Errorf("namespace range check failed for row %d: %w", row, err)
	}
	if outside {
		return nil, ErrNamespaceOutsideRange
	}

	proof, err := tree.ProveRange(fromCol, toCol)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof for row %d, range %d-%d: %w", row, fromCol, toCol, err)
	}

	return &proof, nil
}
