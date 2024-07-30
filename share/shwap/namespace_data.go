package shwap

import (
	"errors"
	"fmt"
	"io"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// NamespaceDataName is the name identifier for the namespace data container.
const NamespaceDataName = "nd_v0"

// NamespaceData stores collections of RowNamespaceData, each representing shares and their proofs
// within a namespace.
type NamespaceData []RowNamespaceData

// Flatten combines all shares from all rows within the namespace into a single slice.
func (nd NamespaceData) Flatten() []share.Share {
	var shares []share.Share
	for _, row := range nd {
		shares = append(shares, row.Shares...)
	}
	return shares
}

// Validate checks the integrity of the NamespaceData against a provided root and namespace.
func (nd NamespaceData) Validate(root *share.AxisRoots, namespace share.Namespace) error {
	rowIdxs := share.RowsWithNamespace(root, namespace)
	if len(rowIdxs) != len(nd) {
		return fmt.Errorf("expected %d rows, found %d rows", len(rowIdxs), len(nd))
	}

	for i, row := range nd {
		if err := row.Validate(root, namespace, rowIdxs[i]); err != nil {
			return fmt.Errorf("validating row: %w", err)
		}
	}
	return nil
}

// ReadFrom reads the binary form of NamespaceData from the provided reader.
func (nd *NamespaceData) ReadFrom(reader io.Reader) (int64, error) {
	var data []RowNamespaceData
	var n int64
	for {
		var pbRow pb.RowNamespaceData
		nn, err := serde.Read(reader, &pbRow)
		n += int64(nn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// all rows have been read
				*nd = data
				return n, nil
			}
			return n, err
		}
		row := RowNamespaceDataFromProto(&pbRow)
		data = append(data, row)
	}
}

// WriteTo writes the binary form of NamespaceData to the provided writer.
func (nd NamespaceData) WriteTo(writer io.Writer) (int64, error) {
	var n int64
	for _, row := range nd {
		pbRow := row.ToProto()
		nn, err := serde.Write(writer, pbRow)
		n += int64(nn)
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
