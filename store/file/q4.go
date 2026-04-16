package file

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

// q4 stores the fourth quadrant of the square.
type q4 struct {
	hdr  *headerV0
	file *os.File
}

// createQ4 creates a new file under given FS path and
// writes the Q4 into it out of given EDS.
// It may leave partially written file if any of the writes fail.
func createQ4(
	path string,
	eds *rsmt2d.ExtendedDataSquare,
) error {
	mod := os.O_RDWR | os.O_CREATE | os.O_EXCL // ensure we fail if already exist
	f, err := os.OpenFile(path, mod, filePermissions)
	if err != nil {
		return fmt.Errorf("creating Q4 file: %w", err)
	}

	err = writeQ4File(f, eds)
	if errClose := f.Close(); errClose != nil {
		err = errors.Join(err, fmt.Errorf("closing created Q4 file: %w", errClose))
	}

	return err
}

// writeQ4File full Q4 content into OS File.
func writeQ4File(f *os.File, eds *rsmt2d.ExtendedDataSquare) error {
	// buffering gives us ~4x speed up
	buf := bufio.NewWriterSize(f, writeBufferSize)

	if err := writeQ4(buf, eds); err != nil {
		return fmt.Errorf("writing Q4: %w", err)
	}

	if err := buf.Flush(); err != nil {
		return fmt.Errorf("flushing Q4: %w", err)
	}

	return nil
}

// writeQ4 writes the forth quadrant of the square to the writer. It writes the quadrant in
// row-major order.
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

func validateQ4Size(path string, eds *rsmt2d.ExtendedDataSquare) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	odsSize := int(eds.Width() / 2)
	shareSize := len(eds.GetCell(0, 0))
	expectedSize := shareSize * odsSize * odsSize

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("getting file info: %w", err)
	}
	if info.Size() != int64(expectedSize) {
		return fmt.Errorf("file size mismatch: expected %d, got %d", expectedSize, info.Size())
	}
	return nil
}

// openQ4 opens an existing Q4 file under given FS path.
func openQ4(path string, hdr *headerV0) (*q4, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &q4{
		hdr:  hdr,
		file: f,
	}, nil
}

func (q4 *q4) close() error {
	return q4.file.Close()
}

func (q4 *q4) axisHalf(axisType rsmt2d.Axis, axisIdx int) (shwap.AxisHalf, error) {
	size := q4.hdr.SquareSize()
	q4AxisIdx := axisIdx - size/2
	if q4AxisIdx < 0 {
		return shwap.AxisHalf{}, fmt.Errorf("invalid axis index for Q4: %d", axisIdx)
	}

	axisHalf, err := readAxisHalf(q4.file, axisType, q4AxisIdx, q4.hdr, 0)
	if err != nil {
		return shwap.AxisHalf{}, fmt.Errorf("reading axis half from Q4: %w", err)
	}

	return shwap.AxisHalf{
		Shares:   axisHalf,
		IsParity: true,
	}, nil
}
