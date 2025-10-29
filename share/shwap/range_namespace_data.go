package shwap

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/v6/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"
	nmt_ns "github.com/celestiaorg/nmt/namespace"
	nmt_pb "github.com/celestiaorg/nmt/pb"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// RangeNamespaceData represents a contiguous segment of shares from an original part of the
// data square (ODS) that belong to a specific namespace. It encapsulates both the share data and the
// cryptographic proofs for the incomplete rows necessary to verify the shares' inclusion
// to the respective row roots.
//
// This structure is typically constructed using RangeNamespaceDataFromShares, which extracts
// a rectangular subset of shares from the EDS and populates minimal proof information.
//
// Fields:
//   - Shares: A 2D slice of shares where each inner slice represents a row. The shares must
//     all belong to the same namespace and fall within the [from, to] coordinate range.
//   - FirstIncompleteRowProof: A Proof object (contains proofs for the first row of a data range)
//   - LastIncompleteRowProof: A Proof object (contains proofs for the last row of a data range)
//
// Notes:
//   - Consumers of this struct should ensure that Verify() is called to validate the data
//     against the respective row roots using the populated or generated proofs.
//   - This structure is commonly used in blob availability proofs, light client verification,
//     and namespace-scoped retrieval over the shwap or DAS interfaces.
//   - FirstIncompleteRowProof, LastIncompleteRowProof can be empty
type RangeNamespaceData struct {
	Shares                  [][]libshare.Share `json:"shares,omitempty"`
	FirstIncompleteRowProof *nmt.Proof         `json:"first_row_proof,omitempty"`
	LastIncompleteRowProof  *nmt.Proof         `json:"last_row_proof,omitempty"`
}

// RangeNamespaceDataFromShares constructs a RangeNamespaceData structure from a selection
// of namespaced shares within an ODS.
//
// Parameters:
//   - shares: A 2D slice of shares grouped by rows (each inner slice represents a row).
//     These shares should cover the range between the 'from' and 'to' coordinates, inclusive.
//   - from: The starting coordinate (row, column) of the range within the ODS.
//   - to: The ending coordinate (row, column) of the range (inclusive).
//
// Returns:
//   - A RangeNamespaceData containing the shares and Merkle proofs for the incomplete rows.
//   - An error if the range is invalid or extraction fails.
func RangeNamespaceDataFromShares(
	extendedRowShares [][]libshare.Share,
	from, to SampleCoords,
) (RangeNamespaceData, error) {
	if len(extendedRowShares) == 0 || len(extendedRowShares[0]) == 0 {
		return RangeNamespaceData{}, errors.New("shares contain no rows or empty row")
	}

	numRows := to.Row - from.Row + 1
	if len(extendedRowShares) != numRows {
		return RangeNamespaceData{}, fmt.Errorf("mismatched rows: expected %d vs got %d", numRows, len(extendedRowShares))
	}

	namespace := extendedRowShares[0][from.Col].Namespace()
	odsSize := len(extendedRowShares[0]) / 2
	isMultiRow := numRows > 1
	startsMidRow := from.Col != 0
	endsMidRow := to.Col != odsSize-1

	// We need a start proof if:
	// - The range doesn't start at the beginning of the row
	// - OR it's a single-row range that ends before the row ends
	startProof := startsMidRow || (!isMultiRow && endsMidRow)

	// We need an end proof if:
	// - The range ends before the row ends
	// - AND spans multiple rows
	endProof := endsMidRow && isMultiRow

	var (
		firstIncompleteRowProof *nmt.Proof
		lastIncompleteRowProof  *nmt.Proof
		err                     error
	)

	if startProof {
		endCol := odsSize
		if !isMultiRow {
			endCol = to.Col + 1
		}
		firstIncompleteRowProof, err = GenerateSharesProofs(
			from.Row,
			from.Col,
			endCol,
			odsSize,
			extendedRowShares[0],
		)
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", from.Row, err)
		}
		extendedRowShares[0] = extendedRowShares[0][from.Col:endCol]
	}

	// incomplete to.Col needs a proof for the last row to be computed
	if endProof {
		lastIncompleteRowProof, err = GenerateSharesProofs(
			to.Row,
			0,
			to.Col+1,
			odsSize,
			extendedRowShares[len(extendedRowShares)-1],
		)
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", to.Row, err)
		}
		extendedRowShares[len(extendedRowShares)-1] = extendedRowShares[len(extendedRowShares)-1][:to.Col+1]
	}

	for row, rowShares := range extendedRowShares {
		if len(rowShares) >= odsSize {
			// keep only original data
			extendedRowShares[row] = rowShares[:odsSize]
		}
		for col, shr := range rowShares {
			if !namespace.Equals(shr.Namespace()) {
				return RangeNamespaceData{}, fmt.Errorf("mismatched namespace for share at: row %d, col: %d", row, col)
			}
		}
	}
	return RangeNamespaceData{
		Shares:                  extendedRowShares,
		FirstIncompleteRowProof: firstIncompleteRowProof,
		LastIncompleteRowProof:  lastIncompleteRowProof,
	}, nil
}

// VerifyInclusion checks whether the shares stored within the RangeNamespaceData (`rngdata.Shares`)
// are valid and provably included in the provided rowRoots.
func (rngdata *RangeNamespaceData) VerifyInclusion(
	from SampleCoords,
	to SampleCoords,
	odsSize int,
	roots [][]byte,
) error {
	return rngdata.verifyShares(rngdata.Shares, from, to, odsSize, roots, false)
}

// VerifyNamespace checks whether the shares stored within the RangeNamespaceData (`rngdata.Shares`)
// are valid and provably included in the provided rowRoots. It does exactly the same as `VerifyInclusion`
// with 1 extra condition: it checks whether the provided data are complete under the namespace.
func (rngdata *RangeNamespaceData) VerifyNamespace(
	from SampleCoords,
	to SampleCoords,
	odsSize int,
	roots [][]byte,
) error {
	return rngdata.verifyShares(rngdata.Shares, from, to, odsSize, roots, true)
}

// verifyShares verifies that the provided 2D slice of shares is valid and correctly
// included in the provided row roots using the associated proofs.
//
// This method is used to verify that a contiguous range of shares (`shares`)
// and is valid within the original data square (ODS),
// as identified by the `RangeNamespaceData` receiver.
func (rngdata *RangeNamespaceData) verifyShares(
	shares [][]libshare.Share,
	from SampleCoords,
	to SampleCoords,
	odsSize int,
	expectedRoots [][]byte,
	nsCompleteness bool,
) error {
	if to.Row-from.Row+1 != len(shares) {
		return fmt.Errorf("mismatched number of rows: expected %d vs got %d", to.Row-from.Row+1, len(shares))
	}
	if len(expectedRoots) != len(shares) {
		return fmt.Errorf("mismatched row roots: expected %d vs got %d", len(shares), len(expectedRoots))
	}
	if rngdata.FirstIncompleteRowProof != nil && rngdata.FirstIncompleteRowProof.Start() != from.Col {
		return fmt.Errorf(
			"first col share index mismatch: expected %d vs got %d", from.Col, rngdata.FirstIncompleteRowProof.Start(),
		)
	}
	if rngdata.LastIncompleteRowProof != nil && rngdata.LastIncompleteRowProof.End()-1 != to.Col {
		return fmt.Errorf(
			"last share index mismatch: expected %d vs got %d", to.Col, rngdata.LastIncompleteRowProof.End()-1,
		)
	}

	startIndex, err := SampleCoordsAs1DIndex(from, odsSize)
	if err != nil {
		return err
	}
	endIndex, err := SampleCoordsAs1DIndex(to, odsSize)
	if err != nil {
		return err
	}

	ns, err := ParseNamespace(shares, startIndex, endIndex+1)
	if err != nil {
		return err
	}

	computedRoots := make([][]byte, len(shares))
	firstIncompleteRoot, err := computeRoot(shares[0], ns, rngdata.FirstIncompleteRowProof, nsCompleteness)
	if err != nil {
		return err
	}
	lastIncompleteRoot, err := computeRoot(shares[len(shares)-1], ns, rngdata.LastIncompleteRowProof, nsCompleteness)
	if err != nil {
		return err
	}

	computedRoots[0] = firstIncompleteRoot
	if len(shares) > 1 {
		computedRoots[len(shares)-1] = lastIncompleteRoot
	}

	// Handle rows that don't have namespace proofs (complete rows)
	for i := range computedRoots {
		// Skip rows that already have their roots computed from proofs
		if computedRoots[i] != nil {
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

		// Store the computed root for the row
		computedRoots[i] = root
	}

	for i, root := range expectedRoots {
		if !bytes.Equal(root, computedRoots[i]) {
			return fmt.Errorf("mismatched root for row %d: expected %x, got %x", from.Row+i, root, computedRoots[i])
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
		Shares:                  pbShares,
		FirstIncompleteRowProof: nmtToNmtPbProof(rngdata.FirstIncompleteRowProof),
		LastIncompleteRowProof:  nmtToNmtPbProof(rngdata.LastIncompleteRowProof),
	}
}

func (rngdata *RangeNamespaceData) IsEmpty() bool {
	if rngdata == nil {
		return true
	}
	if rngdata.Shares == nil && rngdata.FirstIncompleteRowProof == nil && rngdata.LastIncompleteRowProof == nil {
		return true
	}
	return false
}

func RangeNamespaceDataFromProto(nd *pb.RangeNamespaceData) (RangeNamespaceData, error) {
	shares := make([][]libshare.Share, len(nd.Shares))

	for i, shr := range nd.Shares {
		shrs, err := SharesFromProto(shr.GetShares())
		if err != nil {
			return RangeNamespaceData{}, err
		}
		shares[i] = shrs
	}

	return RangeNamespaceData{
		Shares:                  shares,
		FirstIncompleteRowProof: pbNmtToNmtProof(nd.FirstIncompleteRowProof),
		LastIncompleteRowProof:  pbNmtToNmtProof(nd.LastIncompleteRowProof),
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

func computeRoot(
	shares []libshare.Share,
	ns libshare.Namespace,
	proof *nmt.Proof, nsCompleteness bool,
) ([]byte, error) {
	// this is an expected behavior.
	if proof == nil {
		return nil, nil
	}

	if len(shares) == 0 {
		return nil, errors.New("empty share list")
	}

	nth := nmt.NewNmtHasher(
		share.NewSHA256Hasher(),
		nmt_ns.ID(ns.Bytes()).Size(),
		true,
	)

	if proof.IsEmptyProof() || proof.IsOfAbsence() {
		return nil, fmt.Errorf("incomplete row proof is invalid")
	}

	// Compute leaf hashes for the namespace merkle tree
	hashes, err := nmt.ComputePrefixedLeafHashes(nth, ns.Bytes(), libshare.ToBytes(shares))
	if err != nil {
		return nil, fmt.Errorf("failed to compute leaf hashes: %w", err)
	}
	// Compute the row root using the namespace merkle tree proof
	return proof.ComputeRootWithBasicValidation(nth, ns.Bytes(), hashes, nsCompleteness)
}

func buildTreeRootFromLeaves(extendedShares [][]byte, rowIndex uint) ([]byte, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(extendedShares)/2), rowIndex)
	for _, shr := range extendedShares {
		if err := tree.Push(shr); err != nil {
			return nil, fmt.Errorf("failed to build tree for row %d: %w", rowIndex, err)
		}
	}
	return tree.Root()
}

func nmtToNmtPbProof(proof *nmt.Proof) *nmt_pb.Proof {
	if proof == nil {
		return nil
	}
	return &nmt_pb.Proof{
		Start:                 int64(proof.Start()),
		End:                   int64(proof.End()),
		Nodes:                 proof.Nodes(),
		LeafHash:              proof.LeafHash(),
		IsMaxNamespaceIgnored: proof.IsMaxNamespaceIDIgnored(),
	}
}

func pbNmtToNmtProof(pbproof *nmt_pb.Proof) *nmt.Proof {
	if pbproof == nil {
		return nil
	}
	proof := nmt.ProtoToProof(*pbproof)
	return &proof
}

// ParseNamespace validates the share range, checks if it only contains one namespace and returns
// that namespace.
// The provided range, defined by startShare and endShare, is end-exclusive.
func ParseNamespace(rawShares [][]libshare.Share, startShare, endShare int) (libshare.Namespace, error) {
	if startShare < 0 {
		return libshare.Namespace{}, fmt.Errorf("start share %d should be positive", startShare)
	}

	if endShare <= 0 {
		return libshare.Namespace{}, fmt.Errorf("end share %d should be greater the 0", endShare)
	}

	if endShare <= startShare {
		return libshare.Namespace{}, fmt.Errorf(
			"end share %d cannot be lower or equal to the starting share %d", endShare, startShare,
		)
	}

	sharesAmount := 0
	for _, shr := range rawShares {
		sharesAmount += len(shr)
	}

	if endShare-startShare != sharesAmount {
		return libshare.Namespace{}, fmt.Errorf(
			"mismatch shares amount expected %d got %d", endShare-startShare, sharesAmount,
		)
	}

	startShareNs := rawShares[0][0].Namespace()
	for i, rowShares := range rawShares {
		for j, sh := range rowShares {
			ns := sh.Namespace()
			if !bytes.Equal(startShareNs.Bytes(), ns.Bytes()) {
				return libshare.Namespace{}, fmt.Errorf(
					"shares range contain different namespaces at index [%d:%d] %v ", i, j, ns,
				)
			}
		}
	}
	return startShareNs, nil
}

// GenerateSharesProofs generates a nmt proof for a range of shares within a specific row.
// It builds a Namespaced Merkle Tree from the provided row shares and creates a proof
// that the shares in the specified column range are valid
//
// Parameters:
//   - row: the row index in the data square
//   - fromCol: the starting column index (inclusive) for the proof range
//   - toCol: the ending column index (exclusive) for the proof range
//   - size: the size of the original data in th row.
//   - rowShares: slice of shares representing the complete row data
//
// Returns a nmt proof that can be used to verify that shares
// in were included in the specified row.
//
// Returns an error if:
//   - tree construction fails when pushing any share
//   - proof generation fails for the specified range.
func GenerateSharesProofs(
	row, fromCol, toCol, size int,
	rowShares []libshare.Share,
) (*nmt.Proof, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size), uint(row))

	for i, shr := range rowShares {
		if err := tree.Push(shr.ToBytes()); err != nil {
			return nil, fmt.Errorf("failed to build tree at share index %d (row %d): %w", i, row, err)
		}
	}

	proof, err := tree.ProveRange(fromCol, toCol)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof for row %d, range %d-%d: %w", row, fromCol, toCol, err)
	}
	return &proof, nil
}
