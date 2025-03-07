package shwap

import (
	"fmt"
	"github.com/pkg/errors"

	"github.com/celestiaorg/celestia-app/v3/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"

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
	roots *share.AxisRoots,
	from, to SampleCoords,
) (RangeNamespaceData, error) {
	if len(shares) == 0 {
		return RangeNamespaceData{}, fmt.Errorf("empty share list")
	}

	odsSize := len(shares[0]) / 2
	rngData := RangeNamespaceData{
		Start:  from.Row,
		Shares: make([][]libshare.Share, len(shares)),
		Proof:  make([]*Proof, len(shares)),
	}
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
		rngData.Shares[i] = rowShares[from.Col:exclusiveEnd]
		rngData.Proof[i] = NewProof(row, &proof, roots)
		// reset from.Col as we are moving to the next row.
		from.Col = 0
		row++
	}
	return rngData, nil
}

// Verify performs a basic validation of the incoming data. It ensures that the response data
// correspond to the request.
func (rngdata *RangeNamespaceData) Verify(
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	dataHash []byte,
) error {
	if len(rngdata.Shares) != len(rngdata.Proof) {
		return fmt.Errorf(
			"mismatch amount of row shares and proofs, %d:%d",
			len(rngdata.Shares), len(rngdata.Proof),
		)
	}
	if rngdata.IsEmpty() {
		return errors.New("empty data")
	}

	if from.Row != rngdata.Start {
		return fmt.Errorf("mismatched row: wanted: %d, got: %d", rngdata.Start, from.Row)
	}
	if from.Col != rngdata.Proof[0].Start() {
		return fmt.Errorf("mismatched col: wanted: %d, got: %d", rngdata.Proof[0].Start(), from.Col)
	}
	if to.Col != rngdata.Proof[len(rngdata.Proof)-1].End() {
		return fmt.Errorf(
			"mismatched col: wanted: %d, got: %d",
			rngdata.Proof[len(rngdata.Proof)-1].End(), to.Col,
		)
	}

	for i, nsData := range rngdata.Shares {
		if rngdata.Proof[i].IsEmptyProof() {
			return fmt.Errorf("nil proof for row: %d", rngdata.Start+i)
		}
		if rngdata.Proof[i].shareProof.IsOfAbsence() {
			return fmt.Errorf("absence proof for row: %d", rngdata.Start+i)
		}

		err := rngdata.Proof[i].VerifyInclusion(nsData, namespace, dataHash)
		if err != nil {
			return fmt.Errorf("%w for row: %d, %w", ErrFailedVerification, rngdata.Start+i, err)
		}
	}
	return nil
}

// Flatten combines all shares from all rows within the namespace into a single slice.
func (nd *RangeNamespaceData) Flatten() []libshare.Share {
	var shares []libshare.Share
	for _, shrs := range nd.Shares {
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
		rowShares := SharesToProto(shr)
		pbShares[i].Shares = rowShares
		pbProofs[i] = rngdata.Proof[i].ToProto()
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
