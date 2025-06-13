package shwap

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/v4/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	nmt_ns "github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/proof"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// RangeNamespaceData represents a contiguous segment of shares from a data square (EDS)
// that belong to a specific namespace. It encapsulates both the share data and the
// cryptographic proofs necessary to verify the shares' inclusion to the respective row roots.
//
// This structure is typically constructed using RangeNamespaceDataFromShares, which extracts
// a rectangular subset of shares from the EDS and populates minimal proof information.
//
// Fields:
//   - Shares: A 2D slice of shares where each inner slice represents a row. The shares must
//     all belong to the same namespace and fall within the [from, to] coordinate range.
//   - Proof: A Proof object (contains proofs for the boundary rows of a data range)
//
// Notes:
//   - Consumers of this struct should ensure that Verify() is called to validate the data
//     against the respective row roots using the populated or generated proofs.
//   - This structure is commonly used in blob availability proofs, light client verification,
//     and namespace-scoped retrieval over the shwap or DAS interfaces.
type RangeNamespaceData struct {
	StartRow int64                      `json:"start_row"`
	EndRow   int64                      `json:"end_row"`
	Shares   [][]libshare.Share         `json:"shares,omitempty"`
	Proof    *proof.IncompleteRowsProof `json:"incomplete_row_proof,omitempty"`
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
func RangeNamespaceDataFromShares(shares [][]libshare.Share, from, to SampleCoords) (*RangeNamespaceData, error) {
	if len(shares) == 0 {
		return nil, errors.New("empty rows share list")
	}
	if len(shares[0]) == 0 {
		return nil, errors.New("empty rows share list")
	}

	numRows := to.Row - from.Row + 1
	if len(shares) != numRows {
		return nil, fmt.Errorf("mismatched rows: expected %d vs got %d", numRows, len(shares))
	}

	namespace := shares[0][from.Col].Namespace()
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

	var (
		incompleteRowProof = &proof.IncompleteRowsProof{}
		err                error
	)

	if startProof {
		endCol := odsSize
		if isSingleRow {
			endCol = to.Col + 1
		}
		incompleteRowProof.FirstIncompleteRowProof, err = proof.GenerateSharesProofs(
			from.Row,
			from.Col,
			endCol,
			odsSize,
			shares[0],
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate proof for row %d: %w", from.Row, err)
		}
		shares[0] = shares[0][from.Col:endCol]
	}

	// incomplete to.Col needs a proof for the last row to be computed
	if endProof {
		incompleteRowProof.LastIncompleteRowProof, err = proof.GenerateSharesProofs(
			to.Row,
			0,
			to.Col+1,
			odsSize,
			shares[len(shares)-1],
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate proof for row %d: %w", to.Row, err)
		}
		shares[len(shares)-1] = shares[len(shares)-1][:to.Col+1]
	}

	for row, rowShares := range shares {
		if len(rowShares) >= odsSize {
			// keep only original data
			shares[row] = rowShares[:odsSize]
		}
		for col, shr := range rowShares {
			if !namespace.Equals(shr.Namespace()) {
				return nil, fmt.Errorf("mismatched namespace for share at: row %d, col: %d", row, col)
			}
		}
	}
	return &RangeNamespaceData{
		StartRow: int64(from.Row),
		EndRow:   int64(to.Row),
		Shares:   shares,
		Proof:    incompleteRowProof,
	}, nil
}

// VerifyInclusion checks whether the shares stored within the RangeNamespaceData (`rngdata.Shares`)
// are valid and provably included in the provided rowRoots.
//
// It validates that:
//   - The internal shares correspond to the given namespace.
//   - The range specified by `from` and `to` matches the start and end coordinates of the shares.
//   - All computed row roots match to the provided roots.
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
func (rngdata *RangeNamespaceData) VerifyInclusion(
	from SampleCoords,
	to SampleCoords,
	roots [][]byte,
) error {
	return rngdata.VerifyShares(rngdata.Shares, from, to, roots, false)
}

// VerifyNamespace checks whether the shares stored within the RangeNamespaceData (`rngdata.Shares`)
// are valid and provably included in the provided rowRoots. It does exactly the same as `VerifyInclusion`
// with 1 extra condition: it checks whether the provided data are complete under the namespace.
func (rngdata *RangeNamespaceData) VerifyNamespace(
	from SampleCoords,
	to SampleCoords,
	roots [][]byte,
) error {
	return rngdata.VerifyShares(rngdata.Shares, from, to, roots, true)
}

// VerifyShares verifies that the provided 2D slice of shares is valid and correctly
// included in the provided row roots using the associated proofs.
//
// This method is used to verify that a contiguous range of shares (`shares`)
// and is valid within the original data square (ODS),
// as identified by the `RangeNamespaceData` receiver.
//
// Parameters:
//   - shares: A 2D slice where each inner slice represents a row of shares for the given range.
//   - from: The inclusive starting SampleCoords (row, col) of the share range.
//   - to: The exclusive ending SampleCoords (row, col) of the share range.
//   - roots: The expected row root hashes to verify inclusion against.
//
// Notes:
//   - The `shares` data passed is used for recomputing proofs where missing. It is extended (erasure-coded)
//     and parsed row-wise to compute the row roots
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
	from SampleCoords,
	to SampleCoords,
	roots [][]byte,
	nsCompleteness bool,
) error {
	if len(shares) == 0 {
		return errors.New("empty rows share list")
	}
	if len(shares[0]) == 0 {
		return errors.New("empty shares list")
	}
	if rngdata.StartRow != int64(from.Row) {
		return fmt.Errorf("mismatched start row index: expected %d vs got %d", from.Row, rngdata.StartRow)
	}
	if rngdata.EndRow != int64(to.Row) {
		return fmt.Errorf("mismatched end row index: expected %d vs got %d", from.Row, rngdata.StartRow)
	}

	namespace := shares[0][0].Namespace()
	numRows := to.Row - from.Row + 1

	if numRows != len(shares) {
		return fmt.Errorf("mismatched number of rows: expected %d vs got %d", numRows, len(shares))
	}
	if len(roots) != len(shares) {
		return fmt.Errorf("mismatched row roots: expected %d vs got %d", len(shares), roots)
	}

	numIncompleteProofs := 0
	if rngdata.Proof != nil {
		if rngdata.Proof.FirstIncompleteRowProof != nil {
			numIncompleteProofs++
		}
		if rngdata.Proof.LastIncompleteRowProof != nil {
			numIncompleteProofs++
		}
	}
	if numRows < numIncompleteProofs {
		return fmt.Errorf(
			"amount of row shares is less than amount of incomplete proofs: %d vs %d", numRows, numIncompleteProofs,
		)
	}

	nth := nmt.NewNmtHasher(
		share.NewSHA256Hasher(),
		nmt_ns.ID(namespace.Bytes()).Size(),
		true,
	)
	proofs := rngdata.Proof

	var nmtProofs []*nmt.Proof
	if proofs != nil {
		nmtProofs = []*nmt.Proof{rngdata.Proof.FirstIncompleteRowProof, rngdata.Proof.LastIncompleteRowProof}
	}

	rowRoots := make([][]byte, numRows)
	for _, nmtProof := range nmtProofs {
		if nmtProof == nil {
			continue
		}

		if nmtProof.IsOfAbsence() {
			return fmt.Errorf("absence proof is not expected")
		}

		var (
			leaves [][]byte // The actual share data to hash
			index  uint     // Index of the row being processed(first or last)
		)

		if nmtProof.Start() > 0 {
			if from.Col != nmtProof.Start() {
				return fmt.Errorf("mismatched start column index: expected %d got %d", from.Col, nmtProof.Start())
			}
			leaves = libshare.ToBytes(shares[0])
		} else {
			if to.Col != nmtProof.End()-1 {
				return fmt.Errorf("mismatched end column index: expected %d got %d", to.Col, nmtProof.End())
			}
			leaves = libshare.ToBytes(shares[numRows-1])
			index = uint(numRows - 1)
		}

		nth.Reset()
		// Compute leaf hashes for the namespace merkle tree
		hashes, err := nmt.ComputePrefixedLeafHashes(nth, namespace.Bytes(), leaves)
		if err != nil {
			return fmt.Errorf("failed to compute leaf hashes: %w", err)
		}

		// Compute the row root using the namespace merkle tree proof
		root, err := nmtProof.ComputeRootWithBasicValidation(nth, namespace.Bytes(), hashes, nsCompleteness)
		if err != nil {
			return fmt.Errorf("failed to compute root with leaf hashes: %w", err)
		}
		rowRoots[index] = root
	}

	// Handle rows that don't have namespace proofs (complete rows)
	for i := range rowRoots {
		// Skip rows that already have their roots computed from proofs
		if rowRoots[i] != nil {
			continue
		}

		extendedRowShares, err := share.ExtendShares(shares[i])
		if err != nil {
			return fmt.Errorf("failed to extend shares: %w", err)
		}

		// Build the row root from the extended shares
		root, err := buildTreeRootFromLeaves(libshare.ToBytes(extendedRowShares), uint(from.Row)+uint(i))
		if err != nil {
			return fmt.Errorf("failed to build shares proof: %w", err)
		}

		// Store the computed root for this row
		rowRoots[i] = root
	}

	for i, root := range roots {
		if !bytes.Equal(root, rowRoots[i]) {
			return fmt.Errorf("mismatched root for row %d: expected %x, got %x", from.Row+i, root, rowRoots[i])
		}
	}
	return nil
}

// Flatten returns a single slice containing all shares from all rows within the namespace.
//
// If the RangeNamespaceData is empty (i.e., contains no shares), Flatten returns nil.
// Otherwise, it returns a flattened slice of libshare.Share combining shares from all rows.
func (rngdata *RangeNamespaceData) Flatten() []libshare.Share {
	var shares []libshare.Share
	for _, shrs := range rngdata.Shares {
		shares = append(shares, shrs...)
	}
	return shares
}

func (rngdata *RangeNamespaceData) ToProto() *pb.RangeNamespaceData {
	pbShares := make([]*pb.RowShares, len(rngdata.Shares))
	for i, shr := range rngdata.Shares {
		pbShares[i] = &pb.RowShares{}
		rowShares := SharesToProto(shr)
		pbShares[i].Shares = rowShares
	}

	return &pb.RangeNamespaceData{
		StartRow: rngdata.StartRow,
		EndRow:   rngdata.EndRow,
		Shares:   pbShares,
		Proof:    rngdata.Proof.ToProto(),
	}
}

func (rngdata *RangeNamespaceData) CleanupData() {
	if rngdata == nil {
		return
	}
	rngdata.Shares = nil
}

func (rngdata *RangeNamespaceData) IsEmpty() bool {
	return rngdata == nil || rngdata.Proof == nil
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

	return &RangeNamespaceData{
		StartRow: nd.StartRow,
		EndRow:   nd.EndRow,
		Shares:   shares,
		Proof:    proof.IncompleteRowsProofFromProto(nd.Proof),
	}, nil
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

func buildTreeRootFromLeaves(shares [][]byte, index uint) ([]byte, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares)/2), index)
	for _, shr := range shares {
		if err := tree.Push(shr); err != nil {
			return nil, fmt.Errorf("failed to build tree for row %d: %w", index, err)
		}
	}
	return tree.Root()
}
