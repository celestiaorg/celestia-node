package byzantine

import (
	"bytes"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/go-fraud"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	pb "github.com/celestiaorg/celestia-node/share/eds/byzantine/pb"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

const (
	BadEncoding fraud.ProofType = "badencoding"
)

func init() {
	fraud.Register(&BadEncodingProof{})
}

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
) fraud.Proof {

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
func (p *BadEncodingProof) Validate(hdr libhead.Header) error {
	header, ok := hdr.(*header.ExtendedHeader)
	if !ok {
		panic(fmt.Sprintf("invalid header type received during BEFP validation: expected %T, got %T", header, hdr))
	}
	if header.Height() != int64(p.BlockHeight) {
		return fmt.Errorf("incorrect block height during BEFP validation: expected %d, got %d",
			p.BlockHeight, header.Height(),
		)
	}

	if len(header.DAH.RowRoots) != len(header.DAH.ColumnRoots) {
		// NOTE: This should never happen as callers of this method should not feed it with a
		// malformed extended header.
		panic(fmt.Sprintf(
			"invalid extended header: length of row and column roots do not match. (rowRoots=%d) (colRoots=%d)",
			len(header.DAH.RowRoots),
			len(header.DAH.ColumnRoots)),
		)
	}

	// merkleRoots are the roots against which we are going to check the inclusion of the received
	// shares. Changing the order of the roots to prove the shares relative to the orthogonal axis,
	// because inside the rsmt2d library rsmt2d.Row = 0 and rsmt2d.Col = 1
	merkleRoots := header.DAH.RowRoots
	if p.Axis == rsmt2d.Row {
		merkleRoots = header.DAH.ColumnRoots
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

	odsWidth := uint64(len(merkleRoots) / 2)
	codec := share.DefaultRSMT2DCodec()

	// rebuild a row or col.
	rebuiltShares, err := codec.Decode(shares)
	if err != nil {
		return err
	}
	rebuiltExtendedShares, err := codec.Encode(rebuiltShares[0:odsWidth])
	if err != nil {
		return err
	}
	copy(rebuiltShares[odsWidth:], rebuiltExtendedShares)

	tree := wrapper.NewErasuredNamespacedMerkleTree(odsWidth, uint(p.Index))
	for _, share := range rebuiltShares {
		err = tree.Push(share)
		if err != nil {
			return err
		}
	}

	expectedRoot, err := tree.Root()
	if err != nil {
		return err
	}

	// root is a merkle root of the row/col where ErrByzantine occurred
	root := header.DAH.RowRoots[p.Index]
	if p.Axis == rsmt2d.Col {
		root = header.DAH.ColumnRoots[p.Index]
	}

	// comparing rebuilt Merkle Root of bad row/col with respective Merkle Root of row/col from block.
	if bytes.Equal(expectedRoot, root) {
		return fmt.Errorf("invalid %s proof: recomputed Merkle root matches the DAH's row/column root", BadEncoding)
	}
	return nil
}
