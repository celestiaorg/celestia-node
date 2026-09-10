package shwap

import (
	"errors"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v4/share"

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
	var shares []libshare.Share //nolint:prealloc
	for _, row := range nd {
		shares = append(shares, row.Shares...)
	}
	return shares
}

// Length returns the total number of shares in the NamespaceData.
func (nd NamespaceData) Length() int {
	var length int
	for _, row := range nd {
		length += len(row.Shares)
	}
	return length
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

// maxNamespaceDataRows bounds the number of rows read from a stream. A namespace
// can span at most every row of the extended square, so any response exceeding
// this is malformed. Without it a peer can stream length-delimited rows until the
// read deadline, growing the requester's heap unbounded (serde caps each row at
// 1 MiB but not their count).
var maxNamespaceDataRows = 2 * share.MaxSquareSize

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

		if len(ndNew) >= maxNamespaceDataRows {
			return n, fmt.Errorf("namespace data exceeds %d rows", maxNamespaceDataRows)
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

func (nd NamespaceData) IsEmpty() bool {
	return len(nd) == 0
}
