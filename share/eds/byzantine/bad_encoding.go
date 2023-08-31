package byzantine

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/go-fraud"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	pb "github.com/celestiaorg/celestia-node/share/eds/byzantine/pb"
	"github.com/celestiaorg/celestia-node/share/ipld"
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

// Validate ensures that fraud proof is correct.
// Validate checks that provided Merkle Proofs correspond to the shares,
// rebuilds bad row or col from received shares, computes Merkle Root
// and compares it with block's Merkle Root.
func (p *BadEncodingProof) Validate(hdr *header.ExtendedHeader) error {
	if hdr.Height() != p.BlockHeight {
		return fmt.Errorf("incorrect block height during BEFP validation: expected %d, got %d",
			p.BlockHeight, hdr.Height(),
		)
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

	// merkleRoots are the roots against which we are going to check the inclusion of the received
	// shares. Changing the order of the roots to prove the shares relative to the orthogonal axis,
	// because inside the rsmt2d library rsmt2d.Row = 0 and rsmt2d.Col = 1
	merkleRoots := hdr.DAH.RowRoots
	if p.Axis == rsmt2d.Row {
		merkleRoots = hdr.DAH.ColumnRoots
	}

	if int(p.Index) >= len(merkleRoots) {
		return fmt.Errorf("invalid %s proof: index out of bounds (%d >= %d)",
			BadEncoding, int(p.Index), len(merkleRoots),
		)
	}

	if len(p.Shares) != len(merkleRoots) {
		// Since p.Shares should contain all the shares from either a row or a
		// column, it should exactly match the number of row roots. In this
		// context, the number of row roots is the width of the extended data
		// square.
		return fmt.Errorf("invalid %s proof: incorrect number of shares %d != %d",
			BadEncoding, len(p.Shares), len(merkleRoots),
		)
	}

	odsWidth := uint64(len(merkleRoots) / 2)
	amount := uint64(0)
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
		return errors.New("fraud: invalid proof: not enough shares provided to reconstruct row/col")
	}

	// verify that Merkle proofs correspond to particular shares.
	shares := make([][]byte, len(merkleRoots))
	for index, shr := range p.Shares {
		if shr == nil {
			continue
		}
		// validate inclusion of the share into one of the DAHeader roots
		if ok := shr.Validate(ipld.MustCidFromNamespacedSha256(merkleRoots[index])); !ok {
			return fmt.Errorf("invalid %s proof: incorrect share received at index %d", BadEncoding, index)
		}
		// NMTree commits the additional namespace while rsmt2d does not know about, so we trim it
		// this is ugliness from NMTWrapper that we have to embrace ¯\_(ツ)_/¯
		shares[index] = share.GetData(shr.Share)
	}

	codec := share.DefaultRSMT2DCodec()

	// We can conclude that the proof is valid in case we proved the inclusion of `Shares` but
	// the row/col can't be reconstructed, or the building of NMTree fails.
	rebuiltShares, err := codec.Decode(shares)
	if err != nil {
		log.Infow("failed to decode shares at height",
			"height", hdr.Height(), "err", err,
		)
		return nil
	}

	rebuiltExtendedShares, err := codec.Encode(rebuiltShares[0:odsWidth])
	if err != nil {
		log.Infow("failed to encode shares at height",
			"height", hdr.Height(), "err", err,
		)
		return nil
	}
	copy(rebuiltShares[odsWidth:], rebuiltExtendedShares)

	tree := wrapper.NewErasuredNamespacedMerkleTree(odsWidth, uint(p.Index))
	for _, share := range rebuiltShares {
		err = tree.Push(share)
		if err != nil {
			log.Infow("failed to build a tree from the reconstructed shares at height",
				"height", hdr.Height(), "err", err,
			)
			return nil
		}
	}

	expectedRoot, err := tree.Root()
	if err != nil {
		log.Infow("failed to build a tree root at height",
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
		return fmt.Errorf("invalid %s proof: recomputed Merkle root matches the DAH's row/column root", BadEncoding)
	}
	return nil
}
