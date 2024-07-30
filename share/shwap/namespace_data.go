package shwap

import (
	"errors"
	"fmt"
	"io"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// NamespacedData stores collections of RowNamespaceData, each representing shares and their proofs
// within a namespace.
type NamespacedData []RowNamespaceData

// Flatten combines all shares from all rows within the namespace into a single slice.
func (nd NamespacedData) Flatten() []share.Share {
	var shares []share.Share
	for _, row := range nd {
		shares = append(shares, row.Shares...)
	}
	return shares
}

// Validate checks the integrity of the NamespacedData against a provided root and namespace.
func (nd NamespacedData) Validate(root *share.AxisRoots, namespace share.Namespace) error {
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

// ReadFrom reads the binary form of NamespacedData from the provided reader.
func (nd *NamespacedData) ReadFrom(reader io.Reader) error {
	var data []RowNamespaceData
	for {
		var pbRow pb.RowNamespaceData
		_, err := serde.Read(reader, &pbRow)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// all rows have been read
				*nd = data
				return nil
			}
			return err
		}
		row := RowNamespaceDataFromProto(&pbRow)
		data = append(data, row)
	}
}
