package file

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
)

type AxisHalf struct {
	Shares   []share.Share
	IsParity bool
}

func (a AxisHalf) Extended() ([]share.Share, error) {
	if a.IsParity {
		return reconstructShares(codec, a.Shares)
	}
	return extendShares(codec, a.Shares)
}

func extendShares(codec Codec, original []share.Share) ([]share.Share, error) {
	if len(original) == 0 {
		return nil, fmt.Errorf("original shares are empty")
	}

	sqLen := len(original) * 2
	shareSize := len(original[0])

	enc, err := codec.Encoder(sqLen)
	if err != nil {
		return nil, fmt.Errorf("encoder: %w", err)
	}

	shares := make([]share.Share, sqLen)
	copy(shares, original)
	for i := len(original); i < len(shares); i++ {
		shares[i] = make([]byte, shareSize)
	}

	err = enc.Encode(shares)
	if err != nil {
		return nil, fmt.Errorf("encoding: %w", err)
	}
	return shares, nil
}

func reconstructShares(codec Codec, parity []share.Share) ([]share.Share, error) {
	if len(parity) == 0 {
		return nil, fmt.Errorf("parity shares are empty")
	}

	sqLen := len(parity) * 2

	enc, err := codec.Encoder(sqLen)
	if err != nil {
		return nil, fmt.Errorf("encoder: %w", err)
	}

	shares := make([]share.Share, sqLen)
	for i := sqLen / 2; i < sqLen; i++ {
		shares[i] = parity[i-sqLen/2]
	}

	err = enc.Reconstruct(shares)
	if err != nil {
		return nil, fmt.Errorf("reconstructing: %w", err)
	}
	return shares, nil
}
