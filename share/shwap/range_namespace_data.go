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
	Start         int `json:"start"`
	NamespaceData `json:"namespace_data"`
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
	nsData := make([]RowNamespaceData, 0, len(shares))
	rngData := RangeNamespaceData{Start: from.Row}
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
		dataRootProof := NewProof(row, &proof, roots)
		nsData = append(nsData,
			RowNamespaceData{Shares: rowShares[from.Col:exclusiveEnd], Proof: dataRootProof},
		)

		// reset from.Col as we are moving to the next row.
		from.Col = 0
		row++
	}

	rngData.NamespaceData = nsData
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
	if rngdata.IsEmpty() {
		return errors.New("empty data")
	}
	if rngdata.NamespaceData[0].Proof.IsEmptyProof() {
		return errors.New("empty proof")
	}

	if from.Row != rngdata.Start {
		return fmt.Errorf("mismatched row: wanted: %d, got: %d", rngdata.Start, from.Row)
	}
	if from.Col != rngdata.NamespaceData[0].Proof.Start() {
		return fmt.Errorf("mismatched col: wanted: %d, got: %d", rngdata.NamespaceData[0].Proof.Start(), from.Col)
	}
	if to.Col != rngdata.NamespaceData[len(rngdata.NamespaceData)-1].Proof.End() {
		return fmt.Errorf(
			"mismatched col: wanted: %d, got: %d",
			rngdata.NamespaceData[len(rngdata.NamespaceData)-1].Proof.End(), to.Col,
		)
	}

	for i, nsData := range rngdata.NamespaceData {
		if nsData.Proof.IsEmptyProof() {
			return fmt.Errorf("nil proof for row: %d", rngdata.Start+i)
		}
		if nsData.Proof.shareProof.IsOfAbsence() {
			return fmt.Errorf("absence proof for row: %d", rngdata.Start+i)
		}

		err := nsData.Proof.VerifyInclusion(nsData.Shares, namespace, dataHash)
		if err != nil {
			return fmt.Errorf("%w for row: %d, %w", ErrFailedVerification, rngdata.Start+i, err)
		}
	}
	return nil
}

func (rngdata *RangeNamespaceData) IsEmpty() bool {
	return len(rngdata.NamespaceData) == 0
}

func (rngdata *RangeNamespaceData) ToProto() *pb.RangeNamespaceData {
	return &pb.RangeNamespaceData{
		Start:              int32(rngdata.Start),
		RangeNamespaceData: rngdata.NamespaceData.ToProto(),
	}
}

func (rngdata *RangeNamespaceData) CleanupData() {
	for i := range rngdata.NamespaceData {
		rngdata.NamespaceData[i].Shares = nil
	}
}

func RangeNamespaceDataFromProto(nd *pb.RangeNamespaceData) (*RangeNamespaceData, error) {
	data, err := NamespaceDataFromProto(nd.RangeNamespaceData)
	if err != nil {
		return nil, err
	}
	return &RangeNamespaceData{Start: int(nd.Start), NamespaceData: data}, nil
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
