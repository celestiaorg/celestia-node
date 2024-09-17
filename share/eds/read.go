package eds

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-node/share"
)

// ReadAccessor reads up EDS out of the io.Reader until io.EOF and provides.
func ReadAccessor(ctx context.Context, reader io.Reader, root *share.AxisRoots) (*Rsmt2D, error) {
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
	return rsmt2d, nil
}

// ReadShares reads shares from the provided io.Reader until EOF. If EOF is reached, the remaining shares
// are populated as tail padding shares. Provided reader must contain shares in row-major order.
func ReadShares(r io.Reader, shareSize, odsSize int) ([]share.Share, error) {
	shares := make([]share.Share, odsSize*odsSize)
	var total int
	for i := range shares {
		shr := make(share.Share, shareSize)
		n, err := io.ReadFull(r, shr)
		if errors.Is(err, io.EOF) {
			for ; i < len(shares); i++ {
				shares[i] = share.TailPadding()
			}
			return shares, nil
		}
		if err != nil {
			return nil, fmt.Errorf("reading shares: %w, bytes read: %v", err, total+n)
		}

		shares[i] = shr
		total += n
	}
	return shares, nil
}
