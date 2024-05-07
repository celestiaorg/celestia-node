package blob

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/shares"
)

// parser is a helper struct that allows collecting shares and transforming them into the blob.
// it contains all necessary information that is needed to build the blob:
// * position of the blob inside the EDS;
// * blob's length;
// * shares needed to build the blob;
// * extra condition to verify the final blob.
type parser struct {
	index    int
	length   int
	shares   []shares.Share
	verifyFn func(blob *Blob) bool
}

// NOTE: passing shares here needed to detect padding shares(as we do not need this check in
// addShares)
func (p *parser) set(index int, shrs []shares.Share) ([]shares.Share, error) {
	if len(shrs) == 0 {
		return nil, errEmptyShares
	}

	shrs, err := p.skipPadding(shrs)
	if err != nil {
		return nil, err
	}

	if len(shrs) == 0 {
		return nil, errEmptyShares
	}

	// `+=` as index could be updated in `skipPadding`
	p.index += index
	length, err := shrs[0].SequenceLen()
	if err != nil {
		return nil, err
	}

	p.length = shares.SparseSharesNeeded(length)
	return shrs, nil
}

// addShares sets shares until the blob is completed and returns extra shares back.
// we do not need here extra condition to check padding shares as we do not expect it here.
// it is possible only between two blobs.
func (p *parser) addShares(shares []shares.Share) (shrs []shares.Share, isComplete bool) {
	index := -1
	for i, sh := range shares {
		p.shares = append(p.shares, sh)
		if len(p.shares) == p.length {
			index = i
			isComplete = true
			break
		}
	}

	if index == -1 {
		return
	}

	if index+1 >= len(shares) {
		return shrs, true
	}
	return shares[index+1:], true
}

// parse parses shares and creates the Blob.
func (p *parser) parse() (*Blob, error) {
	if p.length != len(p.shares) {
		return nil, fmt.Errorf("invalid shares amount. want:%d, have:%d", p.length, len(p.shares))
	}

	sequence, err := shares.ParseShares(p.shares, true)
	if err != nil {
		return nil, err
	}

	// ensure that sequence length is not 0
	if len(sequence) == 0 {
		return nil, ErrBlobNotFound
	}
	if len(sequence) > 1 {
		return nil, errors.New("unexpected amount of sequences")
	}

	data, err := sequence[0].RawData()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrBlobNotFound
	}

	shareVersion, err := sequence[0].Shares[0].Version()
	if err != nil {
		return nil, err
	}

	blob, err := NewBlob(shareVersion, sequence[0].Namespace.Bytes(), data)
	if err != nil {
		return nil, err
	}
	blob.index = p.index
	return blob, nil
}

// skipPadding skips first share in the range if this share is the Padding share.
func (p *parser) skipPadding(shares []shares.Share) ([]shares.Share, error) {
	if len(shares) == 0 {
		return nil, errEmptyShares
	}

	isPadding, err := shares[0].IsPadding()
	if err != nil {
		return nil, err
	}

	if !isPadding {
		return shares, nil
	}

	// update blob index if we are going to skip one share
	p.index++
	if len(shares) > 1 {
		return shares[1:], nil
	}
	return nil, nil
}

func (p *parser) verify(blob *Blob) bool {
	if p.verifyFn == nil {
		return false
	}
	return p.verifyFn(blob)
}

func (p *parser) isEmpty() bool {
	return p.index == 0 && p.length == 0 && len(p.shares) == 0
}

func (p *parser) reset() {
	p.index = 0
	p.length = 0
	p.shares = nil
}
