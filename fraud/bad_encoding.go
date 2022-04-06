package fraud

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	format "github.com/ipfs/go-ipld-format"
	"github.com/tendermint/tendermint/pkg/consts"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/rsmt2d"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/celestia-node/service/header"
)

type BadEncodingProof struct {
	BlockHeight uint64
	// Shares contains all shares from row/col
	// Shares that did not pass verification in rmst2d will be nil
	// For non-nil shares MerkleProofs are computed
	Shares []*Share
	// Position represents the number of row/col where ErrByzantineRow/ErrByzantineColl occurred
	Position uint8
	isRow    bool
}

func CreateBadEncodingFraudProof(
	ctx context.Context,
	height uint64,
	position uint8,
	isRow bool,
	eds *rsmt2d.ExtendedDataSquare,
	roots [][]byte,
	errShares [][]byte,
	dag format.NodeGetter,
) (Proof, error) {
	shares := make([]*Share, len(errShares))
	for index, share := range errShares {
		if share != nil {
			data := eds.Row(uint(index))
			if isRow {
				data = eds.Col(uint(index))
			}

			tree, err := buildTreeFromLeaves(ctx, data, roots[index], index, dag)
			if err != nil {
				return nil, err
			}
			proof, err := tree.Tree().Prove(int(position))
			if err != nil {
				return nil, err
			}
			/*
				NamespaceID should be replaced with ParitySharesNamespaceID for
				all shares except Q0(all erasure coded shares).
				For EDS 4x4 Q0 is [[0;0],[0;1],[1:0],[1;1]].
				x	x	x	x
				x	x	x	x
				x	x	x	x
				x	x	x	x
			*/
			namespaceID := share[:consts.NamespaceSize]
			if index >= len(errShares)/2 || int(position) >= len(errShares)/2 {
				namespaceID = consts.ParitySharesNamespaceID
			}

			shares[index] = &Share{namespaceID, share, &proof}
		}
	}

	return &BadEncodingProof{
		BlockHeight: height,
		Shares:      shares,
		isRow:       isRow,
		Position:    position,
	}, nil
}

// Type returns type of fraud proof
func (p *BadEncodingProof) Type() ProofType {
	return BadEncoding
}

// Height returns block height
func (p *BadEncodingProof) Height() uint64 {
	return p.BlockHeight
}

// MarshalBinary converts BadEncodingProof to []byte
func (p *BadEncodingProof) MarshalBinary() ([]byte, error) {
	shares := make([]*pb.Share, 0, len(p.Shares))
	for _, share := range p.Shares {
		shares = append(shares, share.ShareToProto())
	}

	badEncodingFraudProof := pb.BadEnconding{
		Height:   p.BlockHeight,
		Shares:   shares,
		Position: uint32(p.Position),
		IsRow:    p.isRow,
	}
	return badEncodingFraudProof.Marshal()
}

func (p *BadEncodingProof) UnmarshalBinary(data []byte) error {
	in := pb.BadEnconding{}
	if err := in.Unmarshal(data); err != nil {
		return err
	}
	befp := UnmarshalBefp(&in)
	*p = *befp

	return nil
}

func UnmarshalBefp(befp *pb.BadEnconding) *BadEncodingProof {
	return &BadEncodingProof{
		BlockHeight: befp.Height,
		Shares:      ProtoToShare(befp.Shares),
	}
}

func (p *BadEncodingProof) Validate(header *header.ExtendedHeader, codec rsmt2d.Codec) error {
	merkleRowRoots := header.DAH.RowsRoots
	merkleColRoots := header.DAH.ColumnRoots
	if int(p.Position) >= len(merkleRowRoots) || int(p.Position) >= len(merkleColRoots) {
		return errors.New("invalid fraud proof: incorrect position of bad row/col")
	}
	if len(merkleRowRoots) != len(merkleColRoots) {
		return errors.New("invalid fraud proof: invalid extended header")
	}
	if len(merkleRowRoots) != len(p.Shares) || len(merkleColRoots) != len(p.Shares) {
		return errors.New("invalid fraud proof: invalid shares")
	}

	roots := merkleRowRoots
	if p.isRow {
		roots = merkleColRoots
	}

	shares := make([][]byte, len(roots))

	// verify that Merkle proofs correspond to particular shares
	for index, share := range p.Shares {
		if share == nil {
			continue
		}
		shares[index] = share.Raw
		if ok := share.Validate(roots[index]); !ok {
			return fmt.Errorf("invalid fraud proof: incorrect share received at position %d", index)
		}
	}

	// rebuild a row/col
	rebuiltShares, err := codec.Decode(shares)
	if err != nil {
		return err
	}
	rebuiltExtendedShares, err := codec.Encode(rebuiltShares)
	if err != nil {
		return err
	}
	rebuiltShares = append(rebuiltShares, rebuiltExtendedShares...)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares) / 2))
	for i, share := range rebuiltShares {
		tree.Push(share, rsmt2d.SquareIndex{Axis: uint(p.Position), Cell: uint(i)})
	}

	merkleRoot := merkleColRoots[p.Position]
	if p.isRow {
		merkleRoot = merkleRowRoots[p.Position]
	}

	// compare Merkle Roots
	if bytes.Equal(tree.Root(), merkleRoot) {
		return errors.New("invalid fraud proof: merkle root matches")
	}

	return nil
}

func buildTreeFromLeaves(
	ctx context.Context,
	leaves [][]byte,
	root []byte,
	axis int,
	dag format.NodeGetter,
) (*wrapper.ErasuredNamespacedMerkleTree, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(leaves) / 2))
	emptyData := bytes.Repeat([]byte{0}, len(leaves[0]))
	for idx, data := range leaves {
		if bytes.Equal(data, emptyData) {
			// if share was not repaired, then we should get it through sampling
			share, err := getShare(ctx, root, idx, len(leaves), dag)
			if err != nil {
				return nil, err
			}
			data = share[consts.NamespaceSize:]
		}
		// Axis is an exteranl shifting(e.g between Rows); Cell - is an internal shifting(e.g inside one col)
		// They are also valid vice versa(Axis - shifting between Cols, Cell - shifting inside row)
		tree.Push(data, rsmt2d.SquareIndex{Axis: uint(axis), Cell: uint(idx)})
	}
	return &tree, nil
}

func getShare(ctx context.Context, root []byte, index int, length int, dag format.NodeGetter) ([]byte, error) {
	cid, err := plugin.CidFromNamespacedSha256(root)
	if err != nil {
		return nil, err
	}
	return ipld.GetLeafData(ctx, cid, uint32(index), uint32(length), dag)
}
