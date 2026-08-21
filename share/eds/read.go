package eds

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
)

// ReadAccessor reads up EDS out of the io.Reader until io.EOF and provides.
func ReadAccessor(ctx context.Context, reader io.Reader, root *share.AxisRoots) (*Rsmt2D, error) {
	odsSize := len(root.RowRoots) / 2
	shares, err := ReadShares(reader, libshare.ShareSize, odsSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read eds from ods bytes: %w", err)
	}

	// verify that the EDS hash matches the expected hash
	rsmt2d, err := Rsmt2DFromShares(shares, odsSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create rsmt2d from shares: %w", err)
	}
	datahash, err := rsmt2d.DataHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate data hash: %w", err)
	}
	if !bytes.Equal(datahash, root.Hash()) {
		return nil, fmt.Errorf(
			"content integrity mismatch: imported root %s doesn't match expected root %s",
			datahash,
			root.Hash(),
		)
	}
	return rsmt2d, nil
}

// ReadShares reads shares from the provided io.Reader until EOF. If EOF is reached on a share
// boundary, the remaining shares are populated as tail padding shares. Provided reader must contain
// shares in row-major order.
func ReadShares(r io.Reader, shareSize, odsSize int) ([]libshare.Share, error) {
	shares := make([]libshare.Share, odsSize*odsSize)
	rowSize := shareSize * odsSize
	var total int
	for row := range odsSize {
		buf := make([]byte, rowSize)
		n, err := io.ReadFull(r, buf)
		total += n
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, fmt.Errorf("reading shares: %w, bytes read: %v", err, total)
			}
			// a partial share means the stream ended mid-share
			if n%shareSize != 0 {
				return nil, fmt.Errorf("reading shares: %w, bytes read: %v", io.ErrUnexpectedEOF, total)
			}
			if err := fillRow(shares, buf[:n], row, shareSize, odsSize); err != nil {
				return nil, err
			}
			for i := row*odsSize + n/shareSize; i < len(shares); i++ {
				shares[i] = libshare.TailPaddingShare()
			}
			return shares, nil
		}
		if err := fillRow(shares, buf, row, shareSize, odsSize); err != nil {
			return nil, err
		}
	}
	return shares, nil
}

// fillRow parses buf into the given row of shares. The shares alias buf, so it must not be reused.
func fillRow(shares []libshare.Share, buf []byte, row, shareSize, odsSize int) error {
	if len(buf)%shareSize != 0 {
		return fmt.Errorf("row buffer of %d bytes is not share-aligned (share size %d)", len(buf), shareSize)
	}
	nShares := len(buf) / shareSize
	start := row * odsSize
	if row < 0 || start+nShares > len(shares) {
		return fmt.Errorf("row %d writes shares [%d:%d) out of %d", row, start, start+nShares, len(shares))
	}
	for i := range nShares {
		sh, err := libshare.NewShare(buf[i*shareSize : (i+1)*shareSize])
		if err != nil {
			return err
		}
		shares[start+i] = sh
	}
	return nil
}
