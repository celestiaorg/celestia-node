package blob

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/go-square/shares"
)

// parser helps to collect shares and transform them into a blob.
// It can handle only one blob at a time.
type parser struct {
	// index is a position of the blob inside the EDS.
	index int
	// length is an amount of the shares needed to build the blob.
	length int
	// shares is a set of shares to build the blob.
	shares   []shares.Share
	verifyFn func(blob *Blob) bool
}

// set tries to find the first blob's share by skipping padding shares and
// sets the metadata of the blob(index and length)
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

// addShares sets shares until the blob is completed and extra remaining shares back.
// It assumes that the remaining shares required for blob completeness are correct and
// do not include padding shares.
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

// parse ensures that correct amount of shares was collected and create a blob from the existing
// shares.
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

// skipPadding iterates through the shares until non-padding share will be found. It guarantees that
// the returned set of shares will start with non-padding share(or empty set of shares).
func (p *parser) skipPadding(shares []shares.Share) ([]shares.Share, error) {
	if len(shares) == 0 {
		return nil, errEmptyShares
	}

	offset := 0
	for _, sh := range shares {
		isPadding, err := sh.IsPadding()
		if err != nil {
			return nil, err
		}
		if !isPadding {
			break
		}
		offset++
	}
	// set start index
	p.index = offset
	if len(shares) > offset {
		return shares[offset:], nil
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

// reset cleans up parser, so it can be re-used within the same verify functionality.
func (p *parser) reset() {
	p.index = 0
	p.length = 0
	p.shares = nil
}
