package shwap

import (
	"errors"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
)

// NamespaceDataName is the name identifier for the namespace data container.
const namespaceDataName = "nd_v0"

// NamespaceData stores collections of RowNamespaceData, each representing shares and their proofs
// within a namespace.
// NOTE: NamespaceData does not have it protobuf Container representation and its only *streamed*
// as RowNamespaceData. The protobuf might be added as need comes.
type NamespaceData []RowNamespaceData

// Flatten combines all shares from all rows within the namespace into a single slice.
func (nd NamespaceData) Flatten() []libshare.Share {
	var shares []libshare.Share
	for _, row := range nd {
		shares = append(shares, row.Shares...)
	}
	return shares
}

// Verify checks the integrity of the NamespaceData against a provided root and namespace.
func (nd NamespaceData) Verify(root *share.AxisRoots, namespace libshare.Namespace) error {
	rowIdxs, err := share.RowsWithNamespace(root, namespace)
	if err != nil {
		return err
	}
	if len(rowIdxs) != len(nd) {
		return fmt.Errorf("expected %d rows, found %d rows", len(rowIdxs), len(nd))
	}

	for i, row := range nd {
		if err := row.Verify(root, namespace, rowIdxs[i]); err != nil {
			return fmt.Errorf("validating row: %w", err)
		}
	}
	return nil
}

// ReadFrom reads NamespaceData from the provided reader implementing io.ReaderFrom.
// It reads series of length-delimited RowNamespaceData until EOF draining the stream.
func (nd *NamespaceData) ReadFrom(reader io.Reader) (int64, error) {
	var ndNew []RowNamespaceData
	var n int64
	for {
		var rnd RowNamespaceData
		nn, err := rnd.ReadFrom(reader)
		n += nn
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return n, err
		}

		ndNew = append(ndNew, rnd)
	}

	// all rows have been read
	*nd = ndNew
	return n, nil
}

// WriteTo writes the length-delimited protobuf of NamespaceData to the provided writer.
// implementing io.WriterTo.
func (nd NamespaceData) WriteTo(writer io.Writer) (int64, error) {
	var n int64
	for _, rnd := range nd {
		nn, err := rnd.WriteTo(writer)
		n += nn
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
