package shwap

import (
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/crypto/merkle"

	"github.com/celestiaorg/celestia-app/v4/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/proof"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// ErrEmptyRangeNamespaceData is returned when the RangeNamespaceData is empty.
var ErrEmptyRangeNamespaceData = errors.New("empty RangeNamespaceData")

// RangeNamespaceData represents a contiguous segment of shares from a data square (EDS)
// that belong to a specific namespace. It encapsulates both the share data and the
// cryptographic proofs necessary to verify the shares' inclusion in the data root.
//
// This structure is typically constructed using RangeNamespaceDataFromShares, which extracts
// a rectangular subset of shares from the EDS and populates minimal proof information.
//
// Fields:
//   - Start: The row index in the EDS where this range of shares begins.
//   - Shares: A 2D slice of shares where each inner slice represents a row. The shares must
//     all belong to the same namespace and fall within the [from, to] coordinate range.
//   - Proof: A slice of Proof objects (one per row in the range) that enable verification
//     of share inclusion. Each Proof contains:
//   - rowRootProof: A Merkle proof verifying that the row root is included in the data root.
//   - sharesProof: An NMT proof verifying that the shares belong to the requested namespace.
//
// Notes:
//   - For **complete rows** (i.e., full rows of shares with no partial start or end), the
//     `sharesProof` field is left empty when the RangeNamespaceData is created. This is
//     because complete row proofs are not needed at construction time and can be computed
//     lazily during verification.
//   - For **incomplete rows** (e.g., where the range starts mid-row or ends mid-row), the
//     `sharesProof` is computed eagerly during construction via `generateSharesProofs`.
//   - Consumers of this struct should ensure that Verify() is called to validate the data
//     against the root using the populated or generated proofs.
//   - This structure is commonly used in blob availability proofs, light client verification,
//     and namespace-scoped retrieval over the shwap or DAS interfaces.
type RangeNamespaceData struct {
	Start  int                `json:"start"`
	Shares [][]libshare.Share `json:"shares,omitempty"`
	Proof  []*Proof           `json:"proof"`

	// DataRootProof is a new optimized data root inclusion proofs that
	// will change Proof field after API deprecation.
	// We can keep it private so it won't be marshaled/unmarshalled
	dataRootProof *proof.DataRootProof
}

// RangeNamespaceDataFromShares constructs a RangeNamespaceData structure from a selection
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
func RangeNamespaceDataFromShares(
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
	odsSize := len(axisRoots.RowRoots) / 2

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
	row := from.Row
	col := from.Col
	for i := range numRows {
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
			sharesProof:  &nmt.Proof{},
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
		rngData.Proof[0].sharesProof = sharesProofs
	}
	// incomplete to.Col needs a proof for the last row to be computed
	if endProof {
		sharesProofs, err := generateSharesProofs(to.Row, 0, to.Col+1, odsSize, namespace, shares[numRows-1])
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", to.Row, err)
		}
		rngData.Proof[numRows-1].sharesProof = sharesProofs
	}
	return rngData, nil
}

func RangeNamespaceDataFromSharesV1(
	shares [][]libshare.Share,
	namespace libshare.Namespace,
	axisRoots *share.AxisRoots,
	from, to SampleCoords,
) (RangeNamespaceData, error) {
	if len(shares) == 0 {
		return RangeNamespaceData{}, fmt.Errorf("empty share list")
	}
	odsSize := len(axisRoots.RowRoots) / 2
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
	var (
		leftProof  *nmt.Proof
		rightProof *nmt.Proof
		err        error
	)

	if startProof {
		endCol := odsSize
		if isSingleRow {
			endCol = to.Col + 1
		}
		leftProof, err = generateSharesProofs(from.Row, from.Col, endCol, odsSize, namespace, shares[0])
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", from.Row, err)
		}
		shares[0] = shares[0][from.Col:endCol]
	}

	// incomplete to.Col needs a proof for the last row to be computed
	if endProof {
		rightProof, err = generateSharesProofs(to.Row, 0, to.Col+1, odsSize, namespace, shares[len(shares)-1])
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", to.Row, err)
		}
		shares[len(shares)-1] = shares[len(shares)-1][:to.Col+1]
	}

	for row, rowShares := range shares {
		// keep only original data
		if len(rowShares) >= odsSize {
			rowShares = rowShares[:odsSize]
			shares[row] = rowShares
		}
		for col, shr := range rowShares {
			if !namespace.Equals(shr.Namespace()) {
				return RangeNamespaceData{},
					fmt.Errorf("targeted namespace was not found in share at {Row: %d, Col: %d}",
						row, col,
					)
			}
		}
	}

	dataRootProof := proof.NewDataRootProof(leftProof, rightProof, axisRoots, int64(from.Row), int64(to.Row+1))
	return RangeNamespaceData{
		Shares:        shares,
		dataRootProof: dataRootProof,
	}, nil
}

// Verify checks whether the shares stored within the RangeNamespaceData (`rngdata.Shares`)
// are valid and provably included in the data root (`dataHash`) for the specified namespace.
//
// It validates that:
//   - The internal shares correspond to the given namespace.
//   - The range specified by `from` and `to` matches the start and end coordinates of the shares.
//   - All share proofs are present, correctly formed, and verify against the provided data root.
//
// This is a convenience wrapper around VerifyShares that reuses the internal share set (`rngdata.Shares`).
//
// Parameters:
//   - namespace: The namespace to which the shares should belong.
//   - from: The starting SampleCoords of the share range (inclusive).
//   - to: The ending SampleCoords of the share range (exclusive).
//   - dataHash: The expected data root hash that the shares should verify against.
//
// Returns:
//   - nil if the shares are valid and the proofs confirm their inclusion in the data root.
//   - An error if the shares are malformed, proofs are missing/invalid, or verification fails.
//
// Notes:
//   - If the receiver (`rngdata`) is nil, calling this method will panic. Use defensive checks if needed.
//   - Any empty or mismatched internal proof data will lead to an error.
func (rngdata *RangeNamespaceData) Verify(
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	dataHash []byte,
) error {
	return rngdata.VerifyShares(rngdata.Shares, namespace, from, to, dataHash)
}

// VerifyShares verifies that the provided 2D slice of shares is valid and correctly
// included in the original data root for the specified namespace, using the associated proofs.
//
// This method is used to verify that a contiguous range of shares (`shares`) belonging to a
// namespace exists and is valid within the original data square (ODS), as identified by the
// `RangeNamespaceData` receiver.
//
// Parameters:
//   - shares: A 2D slice where each inner slice represents a row of shares for the given range.
//   - namespace: The namespace ID these shares are expected to belong to.
//   - from: The inclusive starting SampleCoords (row, col) of the share range.
//   - to: The exclusive ending SampleCoords (row, col) of the share range.
//   - dataHash: The expected data root hash to verify inclusion against.
//
// Requirements:
//   - `shares` must match the number of rows defined in the `RangeNamespaceData.Proof`.
//   - The `from.Row` must match `rngdata.Start`.
//   - The `from.Col` and `to.Col` must align with the start and end of the underlying proof ranges.
//   - Each proof and corresponding share row must be valid and non-empty.
//
// Notes:
//   - The `shares` data passed is used for recomputing proofs where missing. It is extended (erasure-coded)
//     and parsed row-wise to fill in missing `shareProof`s.
//   - Empty or absent proofs or mismatched share counts will cause verification to fail.
//
// Returns an error if:
//   - The data or proofs are malformed or incomplete
//   - Namespace share proof validation fails
//   - Coordinates or share lengths do not align with proof metadata
//
// Panics: This method will not panic but will return descriptive errors on all failure conditions.
func (rngdata *RangeNamespaceData) VerifyShares(
	shares [][]libshare.Share,
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	dataHash []byte,
) error {
	if rngdata.IsEmpty() {
		return ErrEmptyRangeNamespaceData
	}
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
		if !proofs[i].sharesProof.IsEmptyProof() {
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

		proofs[i].sharesProof = rowNamespaceData.Proof
		row++
	}
	if len(shares) != len(proofs) {
		return fmt.Errorf(
			"mismatch amount of row shares and proofs, %d:%d",
			len(shares), len(rngdata.Proof),
		)
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
		if proofs[i].sharesProof.IsOfAbsence() {
			return fmt.Errorf("absence proof for row: %d", rngdata.Start+i)
		}

		err := proofs[i].VerifyInclusion(rowShares, namespace, dataHash)
		if err != nil {
			return fmt.Errorf("%w for row: %d, %w", ErrFailedVerification, rngdata.Start+i, err)
		}
	}
	return nil
}

func (rngdata *RangeNamespaceData) VerifySharesV1(
	shares [][]libshare.Share,
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	dataHash []byte,
) error {
	if len(shares) == 0 {
		return ErrEmptyRangeNamespaceData
	}

	if rngdata.dataRootProof == nil {
		return errors.New("data root proof is empty")
	}

	count := int64(0)
	for row, rowShares := range shares {
		for col, shr := range rowShares {
			if !namespace.Equals(shr.Namespace()) {
				return fmt.Errorf("targeted namespace was not found in share at {Row: %d, Col: %d}",
					row, col,
				)
			}
			count++
		}
	}

	odsSize := rngdata.dataRootProof.ODSSize()
	start, err := SampleCoordsAs1DIndex(from, int(odsSize))
	if err != nil {
		return fmt.Errorf("unable to calcluate start index: %w", err)
	}
	end, err := SampleCoordsAs1DIndex(to, int(odsSize))
	if err != nil {
		return fmt.Errorf("unable to calcluate end index: %w", err)
	}

	proofStartIndex := rngdata.dataRootProof.Start()
	proofEndIndex := rngdata.dataRootProof.End()
	// compare shares number
	if (proofEndIndex - proofStartIndex + 1) != count {
		return fmt.Errorf("mismatched amount of shares to proven: wanted: %d, got: %d",
			proofEndIndex-proofStartIndex+1, count,
		)
	}

	// verify whether indexes matches
	if int64(start) != proofStartIndex || int64(end) != proofEndIndex {
		return fmt.Errorf("mismatched range indexes. want: [%d, %d], got: [%d, %d]",
			start, end, proofStartIndex, proofEndIndex,
		)
	}

	if proof := rngdata.dataRootProof.LeftProof(); proof != nil {
		if proof.IsOfAbsence() {
			return errors.New("range data does not support absence proofs")
		}
		if proof.Start() != from.Col {
			return fmt.Errorf("mismatched start col: wanted: %d, got: %d", from.Col, proof.Start())
		}
	}

	if proof := rngdata.dataRootProof.RightProof(); proof != nil {
		if proof.IsOfAbsence() {
			return errors.New("range data does not support absence proofs")
		}
		if proof.End()-1 != to.Col {
			return fmt.Errorf("mismatched end col: wanted: %d, got: %d", to.Col, proof.End())
		}
	}
	return rngdata.dataRootProof.VerifyInclusion(shares, dataHash)
}

// Flatten returns a single slice containing all shares from all rows within the namespace.
//
// If the RangeNamespaceData is empty (i.e., contains no shares), Flatten returns nil.
// Otherwise, it returns a flattened slice of libshare.Share combining shares from all rows.
func (rngdata *RangeNamespaceData) Flatten() []libshare.Share {
	if rngdata.IsEmpty() {
		return nil
	}
	var shares []libshare.Share
	for _, shrs := range rngdata.Shares {
		shares = append(shares, shrs...)
	}
	return shares
}

// IsEmpty checks if the RangeNamespaceData is empty, meaning it contains no shares or proofs.
//
// Returns:
//   - true if the RangeNamespaceData is nil or has no shares and proofs.
//   - false otherwise.
func (rngdata *RangeNamespaceData) IsEmpty() bool {
	return rngdata == nil || (len(rngdata.Shares) == 0 && len(rngdata.Proof) == 0)
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
	if rngdata.IsEmpty() {
		return
	}
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
