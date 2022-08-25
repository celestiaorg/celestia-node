package fraud

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/pkg/consts"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/rsmt2d"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/ipld"
	ipld_pb "github.com/celestiaorg/celestia-node/ipld/pb"
	"github.com/celestiaorg/celestia-node/ipld/plugin"
)

func init() {
	Register(&BadEncodingProof{})
}

type BadEncodingProof struct {
	headerHash  []byte
	BlockHeight uint64
	// ShareWithProof contains all shares from row or col.
	// Shares that did not pass verification in rsmt2d will be nil.
	// For non-nil shares MerkleProofs are computed.
	Shares []*ipld.ShareWithProof
	// Index represents the row/col index where ErrByzantineRow/ErrByzantineColl occurred.
	Index uint32
	// Axis represents the axis that verification failed on.
	Axis rsmt2d.Axis
}

// CreateBadEncodingProof creates a new Bad Encoding Fraud Proof that should be propagated through network.
// The fraud proof will contain shares that did not pass verification and their relevant Merkle proofs.
func CreateBadEncodingProof(
	hash []byte,
	height uint64,
	errByzantine *ipld.ErrByzantine,
) Proof {

	return &BadEncodingProof{
		headerHash:  hash,
		BlockHeight: height,
		Shares:      errByzantine.Shares,
		Index:       errByzantine.Index,
		Axis:        errByzantine.Axis,
	}
}

// Type returns type of fraud proof.
func (p *BadEncodingProof) Type() ProofType {
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
	shares := make([]*ipld_pb.Share, 0, len(p.Shares))
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
		Shares:      ipld.ProtoToShare(in.Shares),
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
func (p *BadEncodingProof) Validate(header *header.ExtendedHeader) error {
	if header.Height != int64(p.BlockHeight) {
		return errors.New("fraud: incorrect block height")
	}
	merkleRowRoots := header.DAH.RowsRoots
	merkleColRoots := header.DAH.ColumnRoots
	if len(merkleRowRoots) != len(merkleColRoots) {
		// NOTE: This should never happen as callers of this method should not feed it with a
		// malformed extended header.
		panic(fmt.Sprintf(
			"fraud: invalid extended header: length of row and column roots do not match. (rowRoots=%d) (colRoots=%d)",
			len(merkleRowRoots),
			len(merkleColRoots)),
		)
	}
	if int(p.Index) >= len(merkleRowRoots) {
		return fmt.Errorf("fraud: invalid proof: index out of bounds (%d >= %d)", int(p.Index), len(merkleRowRoots))
	}
	if len(merkleRowRoots) != len(p.Shares) {
		return fmt.Errorf("fraud: invalid proof: incorrect number of shares %d != %d", len(p.Shares), len(merkleRowRoots))
	}

	root := merkleRowRoots[p.Index]
	if p.Axis == rsmt2d.Col {
		root = merkleColRoots[p.Index]
	}

	shares := make([][]byte, len(merkleRowRoots))

	// verify that Merkle proofs correspond to particular shares.
	for index, share := range p.Shares {
		if share == nil {
			continue
		}
		shares[index] = share.Share
		if ok := share.Validate(plugin.MustCidFromNamespacedSha256(root)); !ok {
			return fmt.Errorf("fraud: invalid proof: incorrect share received at index %d", index)
		}
	}

	codec := consts.DefaultCodec()
	// rebuild a row or col.
	rebuiltShares, err := codec.Decode(shares)
	if err != nil {
		return err
	}
	rebuiltExtendedShares, err := codec.Encode(rebuiltShares[0 : len(shares)/2])
	if err != nil {
		return err
	}
	rebuiltShares = append(rebuiltShares, rebuiltExtendedShares...)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares) / 2))
	for i, share := range rebuiltShares {
		tree.Push(share, rsmt2d.SquareIndex{Axis: uint(p.Index), Cell: uint(i)})
	}

	// comparing rebuilt Merkle Root of bad row/col with respective Merkle Root of row/col from block.
	if bytes.Equal(tree.Root(), root) {
		return errors.New("fraud: invalid proof: recomputed Merkle root matches the DAH's row/column root")
	}

	return nil
}
