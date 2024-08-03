package eds

import (
	"bytes"
	"context"
	"fmt"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/rsmt2d"
	"io"
)

// ReadEDS reads up EDS out of the io.Reader until io.EOF.
// TODO(@Wondertan): Should be come ReadAccessor.
func ReadEDS(ctx context.Context, reader io.Reader, root *share.AxisRoots) (*rsmt2d.ExtendedDataSquare, error) {
	odsSize := len(root.RowRoots) / 2
	shares, err := ReadShares(reader, share.Size, odsSize)
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
	return rsmt2d.ExtendedDataSquare, nil
}

// ReadShares reads shares from the provided io.Reader until EOF. If EOF is reached, the remaining shares
// are populated as padding share. Provided reader must contain shares in row-major order.
func ReadShares(r io.Reader, shareSize, odsSize int) ([]share.Share, error) {
	shares := make([]share.Share, odsSize*odsSize)
	var total int
	for i := range shares {
		shr := make(share.Share, shareSize)
		n, err := io.ReadFull(r, shr)
		if err != nil {
			return nil, fmt.Errorf("reading share: %w, bytes read: %v", err, total+n)
		}
		if n != shareSize {
			return nil, fmt.Errorf("share size mismatch: expected %v, got %v", shareSize, n)
		}
		shares[i] = shr
		total += n
	}
	return shares, nil
}
