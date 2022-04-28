package fraud

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/pkg/consts"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/rsmt2d"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
	"github.com/celestiaorg/celestia-node/ipld"
	ipld_pb "github.com/celestiaorg/celestia-node/ipld/pb"
	"github.com/celestiaorg/celestia-node/service/header"
)

type BadEncodingProof struct {
	BlockHeight uint64
	// ShareWithProof contains all shares from row or col.
	// Shares that did not pass verification in rmst2d will be nil.
	// For non-nil shares MerkleProofs are computed.
	Shares []*ipld.NamespacedShareWithProof
	// Index represents the row/col index where ErrByzantineRow/ErrByzantineColl occurred
	Index uint8
	// isRow shows that verification failed on row
	isRow bool
}

// CreateBadEncodingProof creates a new Bad Encoding Fraud Proof that should be propagated through network
// Fraud Proof will contain shares that did not pass the verification and appropriate to them Merkle Proofs
func CreateBadEncodingProof(
	height uint64,
	shares *ipld.ErrByzantine,
) Proof {

	return &BadEncodingProof{
		BlockHeight: height,
		Shares:      shares.Shares,
		isRow:       shares.IsRow,
		Index:       shares.Index,
	}
}

// Type returns type of fraud proof
func (p *BadEncodingProof) Type() ProofType {
	return BadEncoding
}

// Height returns block height
func (p *BadEncodingProof) Height() uint64 {
	return p.BlockHeight
}

// MarshalBinary converts BadEncodingProof to binary
func (p *BadEncodingProof) MarshalBinary() ([]byte, error) {
	shares := make([]*ipld_pb.Share, 0, len(p.Shares))
	for _, share := range p.Shares {
		shares = append(shares, share.ShareWithProofToProto())
	}

	badEncodingFraudProof := pb.BadEncoding{
		Height: p.BlockHeight,
		Shares: shares,
		Index:  uint32(p.Index),
		IsRow:  p.isRow,
	}
	return badEncodingFraudProof.Marshal()
}

// UnmarshalBinary converts binary to BadEncodingProof
func (p *BadEncodingProof) UnmarshalBinary(data []byte) error {
	in := pb.BadEncoding{}
	if err := in.Unmarshal(data); err != nil {
		return err
	}
	befp := UnmarshalBEFP(&in)
	*p = *befp

	return nil
}

// UnmarshalBEFP converts given data to BadEncodingProof
func UnmarshalBEFP(befp *pb.BadEncoding) *BadEncodingProof {
	return &BadEncodingProof{
		BlockHeight: befp.Height,
		Shares:      ipld.ProtoToShare(befp.Shares),
	}
}

// Validate ensures that Fraud Proof that was created by full node is correct.
// Validate checks that provided Merkle Proofs are corresponded to particular shares,
// rebuilds bad row or col from received shares, computes Merkle Root
// and compares it with block's Merkle Root
func (p *BadEncodingProof) Validate(header *header.ExtendedHeader) error {
	merkleRowRoots := header.DAH.RowsRoots
	merkleColRoots := header.DAH.ColumnRoots
	if int(p.Index) >= len(merkleRowRoots) || int(p.Index) >= len(merkleColRoots) {
		return errors.New("invalid fraud proof: incorrect Index of bad row/col")
	}
	if len(merkleRowRoots) != len(merkleColRoots) {
		return errors.New("invalid fraud proof: invalid extended header")
	}
	if len(merkleRowRoots) != len(p.Shares) || len(merkleColRoots) != len(p.Shares) {
		return errors.New("invalid fraud proof: invalid shares")
	}

	roots := merkleRowRoots[p.Index]
	if !p.isRow {
		roots = merkleColRoots[p.Index]
	}

	shares := make([][]byte, len(merkleRowRoots))

	// verify that Merkle proofs correspond to particular shares
	for index, share := range p.Shares {
		if share == nil {
			continue
		}
		shares[index] = share.Share
		if ok := share.Validate(roots); !ok {
			return fmt.Errorf("invalid fraud proof: incorrect share received at Index %d", index)
		}
	}

	codec := consts.DefaultCodec()
	// rebuild a row or col
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

	// comparing rebuilt Merkle Root of bad row/col with respective Merkle Root of row/col from block
	merkleRoot := merkleColRoots[p.Index]
	if p.isRow {
		merkleRoot = merkleRowRoots[p.Index]
	}

	if bytes.Equal(tree.Root(), merkleRoot) {
		return errors.New("invalid fraud proof: recomputed Merkle root matches the header's row/column root")
	}

	return nil
}
