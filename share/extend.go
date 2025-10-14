package share

import (
	"fmt"

	libshare "github.com/celestiaorg/go-square/v3/share"
)

var codec = DefaultRSMT2DCodec()

// ExtendShares constructs full axis shares from original half axis shares (data to data+parity).
func ExtendShares(original []libshare.Share) ([]libshare.Share, error) {
	if len(original) == 0 {
		return nil, fmt.Errorf("original shares are empty")
	}

	parity, err := codec.Encode(libshare.ToBytes(original))
	if err != nil {
		return nil, fmt.Errorf("encoding: %w", err)
	}

	parityShrs, err := libshare.FromBytes(parity)
	if err != nil {
		return nil, err
	}

	sqLen := len(original) * 2
	shares := make([]libshare.Share, sqLen)
	copy(shares, original)
	copy(shares[sqLen/2:], parityShrs)
	return shares, nil
}

// ReconstructShares reconstructs original data shares from parity (parity to data+parity).
func ReconstructShares(parity []libshare.Share) ([]libshare.Share, error) {
	if len(parity) == 0 {
		return nil, fmt.Errorf("parity shares are empty")
	}

	sqLen := len(parity) * 2
	shares := make([]libshare.Share, sqLen)
	for i := sqLen / 2; i < sqLen; i++ {
		shares[i] = parity[i-sqLen/2]
	}
	shrs, err := codec.Decode(libshare.ToBytes(shares))
	if err != nil {
		return nil, fmt.Errorf("reconstructing: %w", err)
	}

	return libshare.FromBytes(shrs)
}
