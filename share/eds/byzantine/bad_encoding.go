package byzantine

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/v2/pkg/wrapper"
	"github.com/celestiaorg/go-fraud"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	pb "github.com/celestiaorg/celestia-node/share/eds/byzantine/pb"
)

const (
	version = "v0.1"

	BadEncoding fraud.ProofType = "badencoding" + version
)

type BadEncodingProof struct {
	headerHash  []byte
	BlockHeight uint64
	// ShareWithProof contains all shares from row or col.
	// Shares that did not pass verification in rsmt2d will be nil.
	// For non-nil shares MerkleProofs are computed.
	Shares []*ShareWithProof
	// Index represents the row/col index where ErrByzantineRow/ErrByzantineColl occurred.
	Index uint32
	// Axis represents the axis that verification failed on.
	Axis rsmt2d.Axis
}

// CreateBadEncodingProof creates a new Bad Encoding Fraud Proof that should be propagated through
// network. The fraud proof will contain shares that did not pass verification and their relevant
// Merkle proofs.
func CreateBadEncodingProof(
	hash []byte,
	height uint64,
	errByzantine *ErrByzantine,
) fraud.Proof[*header.ExtendedHeader] {
	return &BadEncodingProof{
		headerHash:  hash,
		BlockHeight: height,
		Shares:      errByzantine.Shares,
		Index:       errByzantine.Index,
		Axis:        errByzantine.Axis,
	}
}

// Type returns type of fraud proof.
func (p *BadEncodingProof) Type() fraud.ProofType {
	return BadEncoding
}

// HeaderHash returns block hash.
func (p *BadEncodingProof) HeaderHash() []byte {
	return p.headerHash
}

// Height returns block height.
func (p *BadEncodingProof) Height() uint64 {
	return p.BlockHeight
}

// MarshalBinary converts BadEncodingProof to binary.
func (p *BadEncodingProof) MarshalBinary() ([]byte, error) {
	shares := make([]*pb.Share, 0, len(p.Shares))
	for _, share := range p.Shares {
		shares = append(shares, share.ShareWithProofToProto())
	}

	badEncodingFraudProof := pb.BadEncoding{
		HeaderHash: p.headerHash,
		Height:     p.BlockHeight,
		Shares:     shares,
		Index:      p.Index,
		Axis:       pb.Axis(p.Axis),
	}
	return badEncodingFraudProof.Marshal()
}

// UnmarshalBinary converts binary to BadEncodingProof.
func (p *BadEncodingProof) UnmarshalBinary(data []byte) error {
	in := pb.BadEncoding{}
	if err := in.Unmarshal(data); err != nil {
		return err
	}
	befp := &BadEncodingProof{
		headerHash:  in.HeaderHash,
		BlockHeight: in.Height,
		Shares:      ProtoToShare(in.Shares),
		Index:       in.Index,
		Axis:        rsmt2d.Axis(in.Axis),
	}

	*p = *befp

	return nil
}

var (
	errHeightMismatch          = errors.New("height reported in proof does not match with the header's height")
	errIncorrectIndex          = errors.New("row/col index is more then the roots amount")
	errIncorrectAmountOfShares = errors.New("incorrect amount of shares")
	errIncorrectShare          = errors.New("incorrect share received")
	errNMTTreeRootsMatch       = errors.New("recomputed root matches the DAH root")
)

var invalidProofPrefix = fmt.Sprintf("invalid %s proof", BadEncoding)

// Validate ensures that fraud proof is correct.
// Validate checks that provided Merkle Proofs correspond to the shares,
// rebuilds bad row or col from received shares, computes Merkle Root
// and compares it with block's Merkle Root.
func (p *BadEncodingProof) Validate(hdr *header.ExtendedHeader) error {
	if hdr.Height() != p.BlockHeight {
		log.Debugf("%s: %s. expected block's height: %d, got: %d",
			invalidProofPrefix,
			errHeightMismatch,
			hdr.Height(),
			p.BlockHeight,
		)
		return errHeightMismatch
	}

	if len(hdr.DAH.RowRoots) != len(hdr.DAH.ColumnRoots) {
		// NOTE: This should never happen as callers of this method should not feed it with a
		// malformed extended header.
		panic(fmt.Sprintf(
			"invalid extended header: length of row and column roots do not match. (rowRoots=%d) (colRoots=%d)",
			len(hdr.DAH.RowRoots),
			len(hdr.DAH.ColumnRoots)),
		)
	}

	width := len(hdr.DAH.RowRoots)
	if int(p.Index) >= width {
		log.Debugf("%s:%s (%d >= %d)",
			invalidProofPrefix, errIncorrectIndex, int(p.Index), width,
		)
		return errIncorrectIndex
	}

	if len(p.Shares) != width {
		// Since p.Shares should contain all the shares from either a row or a
		// column, it should exactly match the number of row roots. In this
		// context, the number of row roots is the width of the extended data
		// square.
		log.Infof("%s: %s (%d >= %d)",
			invalidProofPrefix, errIncorrectAmountOfShares, int(p.Index), width,
		)
		return errIncorrectAmountOfShares
	}

	odsWidth := width / 2
	var amount int
	for _, share := range p.Shares {
		if share == nil {
			continue
		}
		amount++
		if amount == odsWidth {
			break
		}
	}

	if amount < odsWidth {
		log.Debugf("%s: %s. not enough shares provided to reconstruct row/col",
			invalidProofPrefix, errIncorrectAmountOfShares)
		return errIncorrectAmountOfShares
	}

	// verify that Merkle proofs correspond to particular shares.
	shares := make([][]byte, width)
	for index, shr := range p.Shares {
		if shr == nil {
			continue
		}
		// validate inclusion of the share into one of the DAHeader roots
		if ok := shr.Validate(hdr.DAH, p.Axis, int(p.Index), index); !ok {
			log.Debugf("%s: %s at index %d", invalidProofPrefix, errIncorrectShare, index)
			return errIncorrectShare
		}
		shares[index] = shr.Share
	}

	codec := share.DefaultRSMT2DCodec()

	// We can conclude that the proof is valid in case we proved the inclusion of `Shares` but
	// the row/col can't be reconstructed, or the building of NMTree fails.
	rebuiltShares, err := codec.Decode(shares)
	if err != nil {
		log.Debugw("failed to decode shares at height",
			"height", hdr.Height(), "err", err,
		)
		return nil
	}

	rebuiltExtendedShares, err := codec.Encode(rebuiltShares[0:odsWidth])
	if err != nil {
		log.Debugw("failed to encode shares at height",
			"height", hdr.Height(), "err", err,
		)
		return nil
	}
	copy(rebuiltShares[odsWidth:], rebuiltExtendedShares)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(odsWidth), uint(p.Index))
	for _, share := range rebuiltShares {
		err = tree.Push(share)
		if err != nil {
			log.Debugw("failed to build a tree from the reconstructed shares at height",
				"height", hdr.Height(), "err", err,
			)
			return nil
		}
	}

	expectedRoot, err := tree.Root()
	if err != nil {
		log.Debugw("failed to build a tree root at height",
			"height", hdr.Height(), "err", err,
		)
		return nil
	}

	// root is a merkle root of the row/col where ErrByzantine occurred
	root := hdr.DAH.RowRoots[p.Index]
	if p.Axis == rsmt2d.Col {
		root = hdr.DAH.ColumnRoots[p.Index]
	}

	// comparing rebuilt Merkle Root of bad row/col with respective Merkle Root of row/col from block.
	if bytes.Equal(expectedRoot, root) {
		log.Debugf("invalid %s proof:%s", BadEncoding, errNMTTreeRootsMatch)
		return errNMTTreeRootsMatch
	}
	return nil
}
