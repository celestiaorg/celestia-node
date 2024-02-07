package file

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

type square [][]share.Share

// ReadEds reads an EDS from the reader and returns it.
func ReadEds(_ context.Context, r io.Reader, size int) (*rsmt2d.ExtendedDataSquare, error) {
	square, err := readShares(share.Size, size, r)
	if err != nil {
		return nil, fmt.Errorf("reading shares: %w", err)
	}

	eds, err := square.eds()
	if err != nil {
		return nil, fmt.Errorf("computing EDS: %w", err)
	}
	return eds, nil
}

// readShares reads shares from the reader and returns a square. It assumes that the reader is
// positioned at the beginning of the shares. It knows the size of the shares and the size of the
// square, so reads from reader are limited to exactly the amount of data required.
func readShares(shareSize, squareSize int, reader io.Reader) (square, error) {
	odsLn := squareSize / 2

	// get pre-allocated square and buffer from memPools
	square := memPools.get(odsLn).square()
	buf := memPools.get(odsLn).getHalfAxis()
	defer memPools.get(odsLn).putHalfAxis(buf)

	for i := 0; i < odsLn; i++ {
		if _, err := reader.Read(buf); err != nil {
			return nil, err
		}

		for j := 0; j < odsLn; j++ {
			copy(square[i][j], buf[j*shareSize:(j+1)*shareSize])
		}
	}

	return square, nil
}

func (s square) size() int {
	return len(s)
}

func (s square) close() error {
	if s != nil {
		memPools.get(s.size()).putSquare(s)
	}
	return nil
}

func (s square) axisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	if s == nil {
		return nil, fmt.Errorf("ods file not in mem")
	}

	if axisIdx >= s.size() {
		return nil, fmt.Errorf("index is out of ods bounds")
	}

	// square stores rows directly in high level slice, so we can return by accessing row by index
	if axisType == rsmt2d.Row {
		return s[axisIdx], nil
	}

	// construct half column from row ordered square
	col := make([]share.Share, s.size())
	for i := 0; i < s.size(); i++ {
		col[i] = s[i][axisIdx]
	}
	return col, nil
}

func (s square) eds() (*rsmt2d.ExtendedDataSquare, error) {
	shrs := make([]share.Share, 0, 4*s.size()*s.size())
	for _, row := range s {
		shrs = append(shrs, row...)
	}

	treeFn := wrapper.NewConstructor(uint64(s.size()))
	return rsmt2d.ComputeExtendedDataSquare(shrs, share.DefaultRSMT2DCodec(), treeFn)
}

func (s square) Reader(hdr *Header) (io.Reader, error) {
	if s == nil {
		return nil, fmt.Errorf("ods file not cached")
	}

	odsR := &bufferedODSReader{
		square: s,
		total:  s.size() * s.size(),
		buf:    bytes.NewBuffer(make([]byte, 0, int(hdr.shareSize))),
	}

	return odsR, nil
}

func (s square) computeAxisHalf(
	ctx context.Context,
	axisType rsmt2d.Axis,
	axisIdx int,
) ([]share.Share, error) {
	shares := make([]share.Share, s.size())

	// extend opposite half of the square while collecting shares for the first half of required axis
	g, ctx := errgroup.WithContext(ctx)
	opposite := oppositeAxis(axisType)
	for i := 0; i < s.size(); i++ {
		i := i
		g.Go(func() error {
			original, err := s.axisHalf(ctx, opposite, i)
			if err != nil {
				return err
			}

			enc, err := codec.Encoder(s.size() * 2)
			if err != nil {
				return fmt.Errorf("encoder: %w", err)
			}

			shards := make([][]byte, s.size()*2)
			copy(shards, original)
			//for j := len(original); j < len(shards); j++ {
			//	shards[j] = make([]byte, len(original[0]))
			//}

			//err = enc.Encode(shards)
			//if err != nil {
			//	return fmt.Errorf("encode: %w", err)
			//}

			target := make([]bool, s.size()*2)
			target[axisIdx] = true

			err = enc.ReconstructSome(shards, target)
			if err != nil {
				return fmt.Errorf("reconstruct some: %w", err)
			}

			shares[i] = shards[axisIdx]
			return nil
		})
	}

	err := g.Wait()
	return shares, err
}

func oppositeAxis(axis rsmt2d.Axis) rsmt2d.Axis {
	if axis == rsmt2d.Col {
		return rsmt2d.Row
	}
	return rsmt2d.Col
}

// bufferedODSReader will reads shares from inMemOds into the buffer.
// It exposes the buffer to be read by io.Reader interface implementation
type bufferedODSReader struct {
	square square
	// current is the amount of shares stored in ods file that have been read from reader. When current
	// reaches total, bufferedODSReader will prevent further reads by returning io.EOF
	current, total int
	buf            *bytes.Buffer
}

func (r *bufferedODSReader) Read(p []byte) (n int, err error) {
	// read shares to the buffer until it has sufficient data to fill provided container or full ods is
	// read
	for r.current < r.total && r.buf.Len() < len(p) {
		x, y := r.current%(r.square.size()), r.current/(r.square.size())
		r.buf.Write(r.square[y][x])
		r.current++
	}
	// read buffer to slice
	return r.buf.Read(p)
}
