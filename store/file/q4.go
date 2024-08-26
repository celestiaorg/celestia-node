package file

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ eds.AccessorStreamer = (*Q4)(nil)

// Q4 implements eds.Accessor as an FS file.
// It stores the Q4 of the EDS and its metadata in file's header.
// It currently implements access only to Q4
// and can be extended to serve the whole EDS as the need comes.
type Q4 struct {
	hdr  *headerV0
	file *os.File
}

// CreateQ4 creates a new file under given FS path and
// writes the Q4 into it out of given EDS.
// It may leave partially written file if any of the writes fail.
func CreateQ4(
	path string,
	roots *share.AxisRoots,
	eds *rsmt2d.ExtendedDataSquare,
) error {
	mod := os.O_RDWR | os.O_CREATE | os.O_EXCL // ensure we fail if already exist
	f, err := os.OpenFile(path, mod, filePermissions)
	if err != nil {
		return fmt.Errorf("creating Q4 file: %w", err)
	}

	hdr := &headerV0{
		fileVersion: fileV0,
		shareSize:   share.Size,
		squareSize:  uint16(eds.Width()),
		datahash:    roots.Hash(),
	}

	err = writeQ4File(f, eds, hdr)
	if errClose := f.Close(); errClose != nil {
		err = errors.Join(err, fmt.Errorf("closing created Q4 file: %w", errClose))
	}

	return err
}

// writeQ4File full Q4 content into OS File.
func writeQ4File(f *os.File, eds *rsmt2d.ExtendedDataSquare, hdr *headerV0) error {
	// buffering gives us ~4x speed up
	buf := bufio.NewWriterSize(f, writeBufferSize)

	if err := writeHeader(buf, hdr); err != nil {
		return fmt.Errorf("writing Q4 header: %w", err)
	}

	if err := writeQ4(buf, eds); err != nil {
		return fmt.Errorf("writing Q4: %w", err)
	}

	if err := buf.Flush(); err != nil {
		return fmt.Errorf("flushing Q4: %w", err)
	}

	return nil
}

// writeQ4 writes the forth quadrant of the square to the writer. It writes the quadrant in row-major
// order.
func writeQ4(w io.Writer, eds *rsmt2d.ExtendedDataSquare) error {
	half := eds.Width() / 2
	for i := range half {
		for j := range half {
			shr := eds.GetCell(i+half, j+half) // TODO: Avoid copying inside GetCell
			_, err := w.Write(shr)
			if err != nil {
				return fmt.Errorf("writing share: %w", err)
			}
		}
	}
	return nil
}

// OpenQ4 opens an existing Q4 file under given FS path.
// It only reads the header with metadata. The other content
// of the File is read lazily.
// If file is empty, the ErrEmptyFile is returned.
// File must be closed after usage.
func OpenQ4(path string) (*Q4, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	hdr, err := readHeader(f)
	if err != nil {
		return nil, err
	}

	return &Q4{
		hdr:  hdr,
		file: f,
	}, nil
}

func (q4 *Q4) Close() error {
	return q4.file.Close()
}

func (q4 *Q4) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (eds.AxisHalf, error) {
	size := q4.Size(ctx)
	q4AxisIdx := axisIdx - size/2
	if q4AxisIdx < 0 {
		return eds.AxisHalf{}, fmt.Errorf("invalid axis index for Q4: %d", axisIdx)
	}

	axisHalf, err := readAxisHalf(q4.file, axisType, axisIdx, q4.hdr, q4.hdr.Offset())
	if err != nil {
		return eds.AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
	}

	return eds.AxisHalf{
		Shares:   axisHalf,
		IsParity: true,
	}, nil
}

func (q4 *Q4) Size(context.Context) int {
	return q4.hdr.SquareSize()
}

func (q4 *Q4) DataHash(context.Context) (share.DataHash, error) {
	return q4.hdr.datahash, nil
}

func (q4 *Q4) AxisRoots(context.Context) (*share.AxisRoots, error) {
	panic("not implemented")
}

func (q4 *Q4) Sample(context.Context, int, int) (shwap.Sample, error) {
	panic("not implemented")
}

func (q4 *Q4) RowNamespaceData(context.Context, share.Namespace, int) (shwap.RowNamespaceData, error) {
	panic("not implemented")
}

func (q4 *Q4) Shares(context.Context) ([]share.Share, error) {
	panic("not implemented")
}

func (q4 *Q4) Reader() (io.Reader, error) {
	panic("not implemented")
}
