package file

import (
	"bytes"
	"context"
	"fmt"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/rsmt2d"
	"golang.org/x/sync/errgroup"
	"io"
)

type odsInMemFile struct {
	inner  *OdsFile
	square [][]share.Share
}

func ReadEds(ctx context.Context, r io.Reader, root share.DataHash) (*rsmt2d.ExtendedDataSquare, error) {
	h, err := ReadHeader(r)
	if err != nil {
		return nil, err
	}

	ods, err := readOdsInMem(h, r)
	if err != nil {
		return nil, err
	}

	eds, err := ods.EDS(ctx)
	if err != nil {
		return nil, fmt.Errorf("computing EDS: %w", err)
	}

	newDah, err := share.NewRoot(eds)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(newDah.Hash(), root) {
		return nil, fmt.Errorf(
			"share: content integrity mismatch: imported root %s doesn't match expected root %s",
			newDah.Hash(),
			root,
		)
	}
	return eds, nil
}

func readOdsInMem(hdr *Header, reader io.Reader) (*odsInMemFile, error) {
	shrLn := int(hdr.shareSize)
	odsLn := int(hdr.squareSize) / 2

	ods := memPools.get(odsLn).getOds()
	buf := memPools.get(odsLn).getHalfAxis()
	defer memPools.get(odsLn).putHalfAxis(buf)

	for i := 0; i < odsLn; i++ {
		if _, err := reader.Read(buf); err != nil {
			return nil, err
		}

		for j := 0; j < odsLn; j++ {
			copy(ods[i][j], buf[j*shrLn:(j+1)*shrLn])
		}
	}

	return &odsInMemFile{square: ods}, nil
}

func (f *odsInMemFile) Size() int {
	f.inner.lock.RLock()
	defer f.inner.lock.RUnlock()
	return len(f.square) * 2
}

func (f *odsInMemFile) Сlose() error {
	f.inner.lock.RLock()
	defer f.inner.lock.RUnlock()
	if f != nil {
		memPools.get(f.Size() / 2).putOds(f.square)
	}
	return nil
}

func (f *odsInMemFile) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	f.inner.lock.RLock()
	defer f.inner.lock.RUnlock()
	if f == nil {
		return nil, fmt.Errorf("ods file not cached")
	}

	if axisIdx >= f.Size()/2 {
		return nil, fmt.Errorf("index is out of ods bounds")
	}
	if axisType == rsmt2d.Row {
		return f.square[axisIdx], nil
	}

	// TODO: this is not efficient, but it is better than reading from file
	shrs := make([]share.Share, f.Size()/2)
	for i := 0; i < f.Size()/2; i++ {
		shrs[i] = f.square[i][axisIdx]
	}
	return shrs, nil
}

func (f *odsInMemFile) EDS(_ context.Context) (*rsmt2d.ExtendedDataSquare, error) {
	shrs := make([]share.Share, 0, f.Size()*f.Size())
	for _, row := range f.square {
		shrs = append(shrs, row...)
	}

	treeFn := wrapper.NewConstructor(uint64(f.Size() / 2))
	return rsmt2d.ComputeExtendedDataSquare(shrs, share.DefaultRSMT2DCodec(), treeFn)
}

func (f *odsInMemFile) Reader() (io.Reader, error) {
	f.inner.lock.RLock()
	defer f.inner.lock.RUnlock()
	if f == nil {
		return nil, fmt.Errorf("ods file not cached")
	}

	odsR := &bufferedODSReader{
		f:   f,
		buf: bytes.NewBuffer(make([]byte, int(f.inner.hdr.shareSize))),
	}

	// write header to the buffer
	_, err := f.inner.hdr.WriteTo(odsR.buf)
	if err != nil {
		return nil, fmt.Errorf("writing header: %w", err)
	}

	return odsR, nil
}

func (f *odsInMemFile) computeAxisHalf(
	ctx context.Context,
	axisType rsmt2d.Axis,
	axisIdx int,
) ([]share.Share, error) {
	shares := make([]share.Share, f.Size()/2)

	// extend opposite half of the square while collecting shares for the first half of required axis
	g, ctx := errgroup.WithContext(ctx)
	opposite := oppositeAxis(axisType)
	for i := 0; i < f.Size()/2; i++ {
		i := i
		g.Go(func() error {
			original, err := f.AxisHalf(ctx, opposite, i)
			if err != nil {
				return err
			}

			enc, err := codec.Encoder(f.Size())
			if err != nil {
				return fmt.Errorf("encoder: %w", err)
			}

			shards := make([][]byte, f.Size())
			copy(shards, original)
			//for j := len(original); j < len(shards); j++ {
			//	shards[j] = make([]byte, len(original[0]))
			//}

			//err = enc.Encode(shards)
			//if err != nil {
			//	return fmt.Errorf("encode: %w", err)
			//}

			target := make([]bool, f.Size())
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

// bufferedODSReader will reads shares from odsInMemFile into the buffer.
// It exposes the buffer to be read by io.Reader interface implementation
type bufferedODSReader struct {
	f *odsInMemFile
	// current is the amount of shares stored in ods file that have been read from reader. When current
	// reaches total, bufferedODSReader will prevent further reads by returning io.EOF
	current, total int
	buf            *bytes.Buffer
}

func (r *bufferedODSReader) Read(p []byte) (n int, err error) {
	// read shares to the buffer until it has sufficient data to fill provided container or full ods is
	// read
	for r.current < r.total && r.buf.Len() < len(p) {
		x, y := r.current%r.f.Size(), r.current/r.f.Size()
		r.buf.Write(r.f.square[y][x])
		r.current++
	}

	// read buffer to slice
	return r.buf.Read(p)
}
