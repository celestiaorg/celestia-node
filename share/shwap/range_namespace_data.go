package shwap

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v2/pkg/wrapper"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
	"github.com/celestiaorg/nmt"
)

// RangedNamespaceDataFromShares builds a range of namespaced data for the given coordinates.
// rawShares is a list of shares relative to the data square needed to build the range;
// namespace is the target namespace for the built range;
// rowIndex is the starting index of the list relative to the data square;
// from is the share index of the first share of the range;
// to is an EXCLUSIVE index of the last share of the range.
func RangedNamespaceDataFromShares(
	rawShares [][]share.Share,
	namespace share.Namespace,
	rowIndex, from, to int,
) (NamespaceData, error) {
	if len(rawShares) == 0 {
		return NamespaceData{}, fmt.Errorf("empty share list")
	}

	odsSize := len(rawShares[0]) / 2

	nsData := make([]RowNamespaceData, 0, len(rawShares))
	for i, shares := range rawShares {
		tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(odsSize), uint(rowIndex))
		rowIndex++

		nmtTree := nmt.New(
			appconsts.NewBaseHashFunc(),
			nmt.NamespaceIDSize(appconsts.NamespaceSize),
			nmt.IgnoreMaxNamespace(true),
		)
		tree.SetTree(nmtTree)

		for _, shr := range shares {
			if err := tree.Push(shr); err != nil {
				return NamespaceData{}, fmt.Errorf("failed to build tree for row %d: %w", rowIndex, err)
			}
		}

		root, err := tree.Root()
		if err != nil {
			return NamespaceData{}, fmt.Errorf("failed to get root for row %d: %w", rowIndex, err)
		}
		if namespace.IsOutsideRange(root, root) {
			return NamespaceData{}, ErrNamespaceOutsideRange
		}

		to := to
		if i != len(rawShares)-1 {
			// `to` will be equal the odsSize for all intermediate rows(only last one will have targeted index)
			to = odsSize
		}

		for offset := range shares[from:to] {
			if !namespace.Equals(share.GetNamespace(shares[from+offset])) {
				return nil, fmt.Errorf("targeted namespace was not found in range at index: %d", offset)
			}
		}

		proof, err := tree.ProveRange(from, to)
		if err != nil {
			return nil, err
		}

		nsData = append(nsData, RowNamespaceData{Shares: shares[from:to], Proof: &proof})

		// set to 0 as we are moving to the next row
		from = 0
	}
	return nsData, nil
}

// RangeNamespaceData embeds `NamespaceData` and contains a contiguous range of shares along with proofs for these shares.
// Additionally, it contains a `Sample` to ensure that received data is valid.
// Please note: The response can contain proofs only if the user specifies this in the request.
type RangeNamespaceData struct {
	data   NamespaceData
	sample Sample
}

func NewRangeNamespaceData(data NamespaceData, sample Sample) RangeNamespaceData {
	return RangeNamespaceData{data: data, sample: sample}
}

// Verify performs a validation of the incoming data. It ensures that the response contains proofs and performs
// data validation in case the user has requested it.
func (rngdata *RangeNamespaceData) Verify(root *share.AxisRoots, req *RangeNamespaceDataID) error {
	// ensure that received sample corresponds to the requested sampleID
	err := rngdata.sample.Validate(root, req.RowIndex, req.ShareIndex)
	if err != nil {
		return fmt.Errorf("RangeNamespaceData: sample validation failed: %w", err)
	}

	// verify that proofs were received
	if len(rngdata.data.Proofs()) == 0 {
		return errors.New("RangeNamespaceData: no proofs provided")
	}

	// short-circuit if user expects to receive proofs-only.
	if req.ProofsOnly {
		// TODO: build a sub-range proof(when NMT allows it) and compare it with the sample proof
		return nil
	}

	// verify shares amount
	rawShares := rngdata.data.Flatten()
	if len(rawShares) != int(req.Length) {
		return fmt.Errorf("RangeNamespaceData: shares amount mismatch: want: %d, got: %d", req.Length, len(rawShares))
	}

	// compare first received sample with the first share in range
	if !bytes.Equal(rngdata.sample.Share, rawShares[0]) {
		return fmt.Errorf("RangeNamespaceData: invalid start share")
	}

	rowStart := req.RowIndex
	for i, row := range rngdata.data {
		verified := row.Proof.VerifyInclusion(
			share.NewSHA256Hasher(),
			req.RangeNamespace.ToNMT(),
			row.Shares,
			root.RowRoots[rowStart+i],
		)
		if !verified {
			return fmt.Errorf("RangeNamespaceData: proof verification failed at row: %d", rowStart+i)
		}
	}
	return nil
}

// RangeNamespaceDataToProto converts RangeNamespaceData to its protobuf representation
func (rngdata *RangeNamespaceData) RangeNamespaceDataToProto() *pb.RangeNamespaceData {
	rowData := make([]*pb.RowNamespaceData, len(rngdata.data))
	for i, row := range rngdata.data {
		rowData[i] = row.ToProto()
	}
	return &pb.RangeNamespaceData{
		Data:   rowData,
		Sample: rngdata.sample.ToProto(),
	}
}

// ProtoToRangeNamespaceData converts protobuf representation to RangeNamespaceData
func ProtoToRangeNamespaceData(data *pb.RangeNamespaceData) RangeNamespaceData {
	rowData := make([]RowNamespaceData, len(data.Data))
	for i, row := range data.Data {
		rowData[i] = RowNamespaceDataFromProto(row)
	}
	sample := SampleFromProto(data.Sample)
	return NewRangeNamespaceData(rowData, sample)
}
