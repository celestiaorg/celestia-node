package shwap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"sync"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
)

// NamespaceDataIDSize defines the total size of a NamespaceDataID in bytes, combining the
// size of a EdsID and the size of a Namespace.
const NamespaceDataIDSize = EdsIDSize + libshare.NamespaceSize

// NamespaceDataID filters the data in the EDS by a specific namespace.
type NamespaceDataID struct {
	// Embedding EdsID to include the block height.
	EdsID
	// DataNamespace will be used to identify the data within the EDS.
	DataNamespace libshare.Namespace
}

// NewNamespaceDataID creates a new NamespaceDataID with the specified parameters. It
// validates the namespace and returns an error if it is invalid.
func NewNamespaceDataID(height uint64, namespace libshare.Namespace) (NamespaceDataID, error) {
	ndid := NamespaceDataID{
		EdsID: EdsID{
			height: height,
		},
		DataNamespace: namespace,
	}

	if err := ndid.Validate(); err != nil {
		return NamespaceDataID{}, err
	}
	return ndid, nil
}

func (ndid NamespaceDataID) Name() string {
	return namespaceDataName
}

// NamespaceDataIDFromBinary deserializes a NamespaceDataID from its binary form. It returns
// an error if the binary data's length does not match the expected size.
func NamespaceDataIDFromBinary(data []byte) (NamespaceDataID, error) {
	if len(data) != NamespaceDataIDSize {
		return NamespaceDataID{},
			fmt.Errorf("invalid NamespaceDataID length: expected %d, got %d", NamespaceDataIDSize, len(data))
	}

	edsID, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return NamespaceDataID{}, fmt.Errorf("error unmarshaling EDSID: %w", err)
	}

	ns, err := libshare.NewNamespaceFromBytes(data[EdsIDSize:])
	if err != nil {
		return NamespaceDataID{}, fmt.Errorf("error unmarshaling namespace: %w", err)
	}

	ndid := NamespaceDataID{
		EdsID:         edsID,
		DataNamespace: ns,
	}
	if err := ndid.Validate(); err != nil {
		return NamespaceDataID{}, err
	}
	return ndid, nil
}

// Equals checks equality of NamespaceDataID.
func (ndid *NamespaceDataID) Equals(other NamespaceDataID) bool {
	return ndid.EdsID.Equals(other.EdsID) && ndid.DataNamespace.Equals(other.DataNamespace)
}

// ReadFrom reads the binary form of NamespaceDataID from the provided reader.
func (ndid *NamespaceDataID) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, NamespaceDataIDSize)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return int64(n), err
	}
	if n != NamespaceDataIDSize {
		return int64(n), fmt.Errorf("NamespaceDataID: expected %d bytes, got %d", NamespaceDataIDSize, n)
	}
	id, err := NamespaceDataIDFromBinary(data)
	if err != nil {
		return int64(n), fmt.Errorf("NamespaceDataIDFromBinary: %w", err)
	}
	*ndid = id
	return int64(n), nil
}

// MarshalBinary encodes NamespaceDataID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (ndid NamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, NamespaceDataIDSize)
	data, err := ndid.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteTo writes the binary form of NamespaceDataID to the provided writer.
func (ndid NamespaceDataID) WriteTo(w io.Writer) (int64, error) {
	data, err := ndid.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// Validate checks if the NamespaceDataID is valid. It checks the validity of the EdsID and the
// DataNamespace.
func (ndid NamespaceDataID) Validate() error {
	if err := ndid.EdsID.Validate(); err != nil {
		return fmt.Errorf("validating RowID: %w", err)
	}

	if err := ndid.DataNamespace.ValidateForData(); err != nil {
		return fmt.Errorf("%w: validating DataNamespace: %w", ErrInvalidID, err)
	}
	return nil
}

// AppendBinary helps in appending the binary form of DataNamespace to the serialized RowID data.
func (ndid NamespaceDataID) AppendBinary(data []byte) ([]byte, error) {
	data, err := ndid.EdsID.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	return append(data, ndid.DataNamespace.Bytes()...), nil
}

// namespaceDataPrefetch is the number of rows fetched concurrently ahead of the stream writer.
// It bounds the memory held while serving a NamespaceData response.
const namespaceDataPrefetch = 4

// ResponseSize returns the memory held while streaming the response: up to namespaceDataPrefetch
// rows of namespace data (shares + proof), each in decoded and encoded form.
func (ndid NamespaceDataID) ResponseSize(edsSize int) int {
	odsLn := edsSize / 2
	rowSize := odsLn*libshare.ShareSize + 2*share.AxisRootSize*int(math.Log2(float64(edsSize)))
	return namespaceDataPrefetch * 2 * rowSize
}

// ResponseReader returns a reader that streams NamespaceData row by row, fetching up to
// namespaceDataPrefetch rows concurrently, so memory use is bounded regardless of the square size.
// The first row is awaited before returning so that accessor errors surface before any data is
// sent. The returned reader implements io.Closer and must be closed before the accessor is.
func (ndid NamespaceDataID) ResponseReader(ctx context.Context, acc Accessor) (io.Reader, error) {
	roots, err := acc.AxisRoots(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get AxisRoots: %w", err)
	}

	rowIdxs, err := share.RowsWithNamespace(roots, ndid.DataNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get row indexes: %w", err)
	}

	r := newNamespaceDataReader(ctx, acc, ndid.DataNamespace, rowIdxs)
	if err := r.next(); err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}

type encodedRow struct {
	data []byte
	err  error
}

// namespaceDataReader reads and encodes RowNamespaceData for each row index with bounded
// concurrency, producing the same wire format as NamespaceData.WriteTo.
type namespaceDataReader struct {
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	sem     chan struct{}
	results []chan encodedRow
	pos     int
	buf     []byte
}

func newNamespaceDataReader(
	ctx context.Context,
	acc Accessor,
	namespace libshare.Namespace,
	rowIdxs []int,
) *namespaceDataReader {
	ctx, cancel := context.WithCancel(ctx)
	r := &namespaceDataReader{
		ctx:     ctx,
		cancel:  cancel,
		sem:     make(chan struct{}, namespaceDataPrefetch),
		results: make([]chan encodedRow, len(rowIdxs)),
	}
	for i := range r.results {
		r.results[i] = make(chan encodedRow, 1)
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for i, idx := range rowIdxs {
			// a slot is released once the row is consumed by Read.
			select {
			case r.sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			r.wg.Add(1)
			go func() {
				defer r.wg.Done()
				r.results[i] <- fetchEncodedRow(ctx, acc, namespace, idx)
			}()
		}
	}()
	return r
}

func fetchEncodedRow(ctx context.Context, acc Accessor, namespace libshare.Namespace, idx int) encodedRow {
	rowData, err := acc.RowNamespaceData(ctx, namespace, idx)
	if err != nil {
		return encodedRow{err: fmt.Errorf("failed to process row %d: %w", idx, err)}
	}
	var buf bytes.Buffer
	if _, err := rowData.WriteTo(&buf); err != nil {
		return encodedRow{err: fmt.Errorf("writing row %d: %w", idx, err)}
	}
	return encodedRow{data: buf.Bytes()}
}

func (r *namespaceDataReader) Read(p []byte) (int, error) {
	if len(r.buf) == 0 {
		if r.pos == len(r.results) {
			return 0, io.EOF
		}
		if err := r.next(); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

// next waits for the next row in order and releases its prefetch slot.
func (r *namespaceDataReader) next() error {
	if r.pos == len(r.results) {
		return nil
	}
	var res encodedRow
	select {
	case res = <-r.results[r.pos]:
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
	r.results[r.pos] = nil
	r.pos++
	<-r.sem
	if res.err != nil {
		return res.err
	}
	r.buf = res.data
	return nil
}

// Close stops outstanding fetches and waits for them to finish.
func (r *namespaceDataReader) Close() error {
	r.cancel()
	r.wg.Wait()
	return nil
}
