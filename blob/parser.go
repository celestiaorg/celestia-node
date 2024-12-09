package blob

import (
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto/merkle"

	"github.com/celestiaorg/celestia-app/v3/pkg/appconsts"
	"github.com/celestiaorg/go-square/v2/inclusion"
	libshare "github.com/celestiaorg/go-square/v2/share"
)

// parser helps to collect shares and transform them into a blob.
// It can handle only one blob at a time.
type parser struct {
	// index is a position of the blob inside the EDS.
	index int
	// length is an amount of the shares needed to build the blob.
	length int
	// shares is a set of shares to build the blob.
	shares   []libshare.Share
	verifyFn func(blob *Blob) bool
}

// set tries to find the first blob's share by skipping padding shares and
// sets the metadata of the blob(index and length)
func (p *parser) set(index int, shrs []libshare.Share) ([]libshare.Share, error) {
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
	length := shrs[0].SequenceLen()
	p.length = libshare.SparseSharesNeeded(length)
	return shrs, nil
}

// addShares sets shares until the blob is completed and extra remaining shares back.
// It assumes that the remaining shares required for blob completeness are correct and
// do not include padding shares.
func (p *parser) addShares(shares []libshare.Share) (shrs []libshare.Share, isComplete bool) {
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

	blobs, err := libshare.ParseBlobs(p.shares)
	if err != nil {
		return nil, err
	}

	if len(blobs) != 1 {
		return nil, errors.New("unexpected amount of blobs during parsing")
	}

	com, err := inclusion.CreateCommitment(blobs[0], merkle.HashFromByteSlices, appconsts.DefaultSubtreeRootThreshold)
	if err != nil {
		return nil, err
	}

	blob := &Blob{Blob: blobs[0], Commitment: com, index: p.index}
	return blob, nil
}

// skipPadding iterates through the shares until non-padding share will be found. It guarantees that
// the returned set of shares will start with non-padding share(or empty set of shares).
func (p *parser) skipPadding(shares []libshare.Share) ([]libshare.Share, error) {
	if len(shares) == 0 {
		return nil, errEmptyShares
	}

	offset := 0
	for _, sh := range shares {
		if !sh.IsPadding() {
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
