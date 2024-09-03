package shwap

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"

	"github.com/celestiaorg/celestia-node/share"
)

// RangeNamespaceData embeds `NamespaceData` and contains a contiguous range of shares along with proofs for these shares.
// Additionally, it includes a `Sample` to ensure that the received data is valid.
// Please note: The response can contain proofs only if the user specifies this in the request.
type RangeNamespaceData struct {
	NamespaceData
	sample *Sample
}

func NewRangeNamespaceData(data NamespaceData, sample *Sample) *RangeNamespaceData {
	return &RangeNamespaceData{NamespaceData: data, sample: sample}
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
	if len(rngdata.Proofs()) == 0 {
		return errors.New("RangeNamespaceData: no proofs provided")
	}

	// short-circuit if user expects to receive proofs-only.
	if req.ProofsOnly {
		return nil
	}

	// verify shares amount
	rawShares := rngdata.Flatten()
	if len(rawShares) != int(req.Length) {
		return fmt.Errorf("RangeNamespaceData: shares amount mismatch: want: %d, got: %d", req.Length, len(rawShares))
	}

	// compare first received sample with the first share in range
	if !bytes.Equal(rngdata.sample.Share, rawShares[0]) {
		return fmt.Errorf("RangeNamespaceData: invalid start share")
	}

	// namespace verification
	for i := range rawShares {
		ns := share.GetNamespace(rawShares[i])
		if !bytes.Equal(ns, req.RangeNamespace) {
			return fmt.Errorf("RangeNamespaceData: namespaces mismatch at index: %d, want: %X, got: %X",
				i, req.RangeNamespace, ns,
			)
		}
	}

	// proofs verification
	err = rngdata.NamespaceData.Validate(root, req.RangeNamespace)
	if err != nil {
		return fmt.Errorf("RangeNamespaceData: proof verification failed: %w", err)
	}
	return nil
}

// RangeNamespaceDataToProto converts RangeNamespaceData to its protobuf representation
func (rngdata *RangeNamespaceData) RangeNamespaceDataToProto() *pb.RangeNamespaceData {
	rowData := make([]*pb.RowNamespaceData, len(rngdata.NamespaceData))
	for i, row := range rngdata.NamespaceData {
		rowData[i] = row.ToProto()
	}
	return &pb.RangeNamespaceData{
		Data:   rowData,
		Sample: rngdata.sample.ToProto(),
	}
}

// ProtoToRangeNamespaceData converts protobuf representation to RangeNamespaceData
func ProtoToRangeNamespaceData(data *pb.RangeNamespaceData) *RangeNamespaceData {
	rowData := make([]RowNamespaceData, len(data.Data))
	for i, row := range data.Data {
		rowData[i] = RowNamespaceDataFromProto(row)
	}
	sample := SampleFromProto(data.Sample)
	return NewRangeNamespaceData(rowData, &sample)
}
