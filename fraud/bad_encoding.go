package fraud

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/celestiaorg/rsmt2d"
	"github.com/tendermint/tendermint/pkg/consts"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
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
	height uint64,
	position uint8,
	isRow bool,
	eds *rsmt2d.ExtendedDataSquare,
	errShares [][]byte,
) (Proof, error) {
	var tree ErasuredNamespacedMerkleTree
	shares := make([]*Share, len(errShares))
	data := make([][]byte, len(eds.Col(uint(0))))

	for index, share := range errShares {
		if share != nil {
			tree = NewErasuredNamespacedMerkleTree(uint64(len(errShares) / 2))
			data = eds.Row(uint(index))
			if isRow {
				data = eds.Col(uint(index))
			}
			if err := buildTreeFromDataRoot(&tree, data, index); err != nil {
				continue
			}

			proof, err := tree.InclusionProof(uint8(position))
			if err != nil {
				return nil, err
			}
			/*
				NamespaceID sholud be replaced with ParitySharesNamespaceID for
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

func (p *BadEncodingProof) Validate(header *header.ExtendedHeader) error {
	merkleRowRoots := header.DAH.RowsRoots
	merkleColRoots := header.DAH.ColumnRoots
	if int(p.Position) >= len(merkleRowRoots) || int(p.Position) >= len(merkleColRoots) {
		return errors.New("invalid fraud proof: incorrect position of data square")
	}
	if len(merkleRowRoots) != len(merkleColRoots) {
		return errors.New("invalid fraud proof: invalid extended header")
	}
	if len(merkleRowRoots) != len(p.Shares) || len(p.Shares) != len(p.Shares) {
		return errors.New("invalid fraud proof: invalid shares")
	}

	var rebuildNeeded bool
	roots := merkleRowRoots
	if p.isRow {
		roots = merkleColRoots
	}

	shares := make([][]byte, len(roots))

	// verify that Merkle proofs correspond to particular shares
	for index, share := range p.Shares {
		if share == nil {
			rebuildNeeded = true
			continue
		}
		shares[index] = share.Raw
		if ok := share.Validate(roots[index]); !ok {
			return fmt.Errorf("invalid fraud proof: incorrect share received at position %d", index)
		}
	}

	if rebuildNeeded {
		// rebuild a row/col
		codec := rsmt2d.NewRSGF8Codec()
		rebuildShares, err := codec.Decode(shares)
		if err != nil {
			return err
		}
		rebuiltExtendedShares, err := codec.Encode(rebuildShares)
		if err != nil {
			return err
		}

		shares = append(
			rebuildShares,
			rebuiltExtendedShares...,
		)

	}

	tree := NewErasuredNamespacedMerkleTree(uint64(len(shares) / 2))
	for i, share := range shares {
		tree.Push(share, rsmt2d.SquareIndex{Axis: uint(p.Position), Cell: uint(i)})
	}

	merkleRoot := merkleColRoots[p.Position]
	if p.isRow {
		merkleRoot = merkleRowRoots[p.Position]
	}

	// compare Merkle Roots
	if !bytes.Equal(tree.Root(), merkleRoot) {
		return errors.New("invalid fraud proof: merkle roots do not match")
	}
	return nil
}

func buildTreeFromDataRoot(tree *ErasuredNamespacedMerkleTree, root [][]byte, index int) error {
	for idx, data := range root {
		if data == nil {
			return errors.New("empty share")
		}
		// error handling is not supposed here because ErasuredNamespacedMerkleTree panics in case of any error
		// TODO: should we rework this mechanism?

		// Axis is an exteranl shifting(e.g between Rows); Cell - is an internal shifting(e.g inside one col)
		// They are also valid vice versa(Axis - shifting between Cols, Cell - shifting inside row)
		tree.Push(data, rsmt2d.SquareIndex{Axis: uint(index), Cell: uint(idx)})

	}
	return nil
}
