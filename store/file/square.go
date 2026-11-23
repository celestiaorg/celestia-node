package file

import (
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

type square [][]libshare.Share

// readSquare reads Shares from the reader and returns a square. It assumes that the reader is
// positioned at the beginning of the Shares. It knows the size of the Shares and the size of the
// square, so reads from reader are limited to exactly the amount of data required.
func readSquare(r io.Reader, shareSize, edsSize int) (square, error) {
	odsLn := edsSize / 2

	shares, err := eds.ReadShares(r, shareSize, odsLn)
	if err != nil {
		return nil, fmt.Errorf("reading shares: %w", err)
	}
	square := make(square, odsLn)
	for i := range square {
		square[i] = shares[i*odsLn : (i+1)*odsLn]
	}
	return square, nil
}

func (s square) reader() (io.Reader, error) {
	if s == nil {
		return nil, fmt.Errorf("ods file not cached")
	}
	getShare := func(rowIdx, colIdx int) (libshare.Share, error) {
		return s[rowIdx][colIdx], nil
	}
	reader := eds.NewShareReader(s.size(), getShare)
	return reader, nil
}

func (s square) size() int {
	return len(s)
}

func (s square) shares() ([]libshare.Share, error) {
	shares := make([]libshare.Share, 0, s.size()*s.size())
	for _, row := range s {
		shares = append(shares, row...)
	}
	return shares, nil
}

func (s square) axisHalf(axisType rsmt2d.Axis, axisIdx int) (shwap.AxisHalf, error) {
	if s == nil {
		return shwap.AxisHalf{}, fmt.Errorf("square is nil")
	}

	if axisIdx >= s.size() {
		return shwap.AxisHalf{}, fmt.Errorf("index is out of square bounds")
	}

	// square stores rows directly in high level slice, so we can return by accessing row by index
	if axisType == rsmt2d.Row {
		row := s[axisIdx]
		return shwap.AxisHalf{
			Shares:   row,
			IsParity: false,
		}, nil
	}

	// construct half column from row ordered square
	col := make([]libshare.Share, s.size())
	for i := range s.size() {
		col[i] = s[i][axisIdx]
	}
	return shwap.AxisHalf{
		Shares:   col,
		IsParity: false,
	}, nil
}
