package shwap

import (
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto/merkle"

	"github.com/celestiaorg/celestia-app/v3/pkg/wrapper"
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

// RangedNamespaceDataFromShares builds a range of namespaced data for the given coordinates:
// shares is a list of shares (grouped by the rows) relative to the data square and
// needed to build the range;
// namespace is the target namespace for the built range;
// from is the coordinates of the first share of the range within the EDS.
// to is the coordinates of the last inclusive share of the range within the EDS.
func RangedNamespaceDataFromShares(
	shares [][]libshare.Share,
	namespace libshare.Namespace,
	incompleteRowRootProofs []*merkle.Proof,
	from, to SampleCoords,
) (RangeNamespaceData, error) {
	if len(shares) == 0 {
		return RangeNamespaceData{}, fmt.Errorf("empty share list")
	}

	odsSize := len(shares[0]) / 2
	incompleteProofSize := 0
	startProof := NeedsStartProof(from, to, odsSize)
	endProof := NeedsEndProof(from, to, odsSize)
	if startProof {
		incompleteProofSize++
	}
	if endProof {
		incompleteProofSize++
	}
	rngData := RangeNamespaceData{
		Start:  from.Row,
		Shares: make([][]libshare.Share, len(shares)),
		Proof:  make([]*Proof, incompleteProofSize),
	}
	for i, row, col := 0, from.Row, from.Col; i < len(shares); i++ {
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

		// reset from.Col as we are moving to the next row.
		col = 0
		row++
	}
	// incomplete from.Col needs a proof for the first row to be computed
	if startProof {
		endCol := odsSize
		if from.Row == to.Row {
			endCol = to.Col + 1
		}
		sharesProofs, err := generateSharesProofs(from.Row, from.Col, endCol, odsSize, namespace, shares[0])
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", from.Row, err)
		}
		rngData.Proof[0] = &Proof{shareProof: sharesProofs, rowRootProof: incompleteRowRootProofs[0]}
	}
	// incomplete to.Col needs a proof for the last row to be computed
	if endProof {
		sharesProofs, err := generateSharesProofs(to.Row, 0, to.Col+1, odsSize, namespace, shares[len(shares)-1])
		if err != nil {
			return RangeNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", to.Row, err)
		}
		rngData.Proof[incompleteProofSize-1] = &Proof{shareProof: sharesProofs, rowRootProof: incompleteRowRootProofs[incompleteProofSize-1]}
	}
	return rngData, nil
}

// Verify verifies the underlying shares are included in the data root.
func (rngdata *RangeNamespaceData) Verify(
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	odsSize int,
	dataHash []byte,
	rowRootProofs []*merkle.Proof,
) error {
	return rngdata.VerifyShares(rngdata.Shares, namespace, from, to, odsSize, dataHash, rowRootProofs)
}

// VerifyShares verifies the passed shares are included in the data root.
// `[][]libshare.Share` is a collection of the row shares from the range
// NOTE: the underlying shares will be ignored even if they are not empty.
func (rngdata *RangeNamespaceData) VerifyShares(
	shares [][]libshare.Share,
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	odsSize int,
	dataHash []byte,
	rowRootProofs []*merkle.Proof,
) error {
	startProof := NeedsStartProof(from, to, odsSize)
	endProof := NeedsEndProof(from, to, odsSize)
	proofs := make([]*Proof, len(shares))
	// copy the incomplete row proofs
	if startProof {
		proofs[0] = rngdata.Proof[0]
	}
	if endProof {
		proofs[len(shares)-1] = rngdata.Proof[len(rngdata.Proof)-1]
	}
	// compute the proofs for the complete rows
	for i, row := 0, from.Row; i < len(shares); i++ {
		// skip the incomplete rows
		if i == 0 && from.Col != 0 {
			continue
		}
		if i == len(shares)-1 && to.Col != odsSize-1 {
			continue
		}

		rowShares := shares[i]
		proof, err := generateSharesProofs(row, 0, odsSize, odsSize, namespace, rowShares)
		if err != nil {
			return fmt.Errorf("failed to generate proof for row %d: %w", row, err)
		}
		proofs[i] = &Proof{shareProof: proof, rowRootProof: rowRootProofs[row]}
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

	if from.Row != rngdata.Start {
		return fmt.Errorf("mismatched row: wanted: %d, got: %d", rngdata.Start, from.Row)
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
