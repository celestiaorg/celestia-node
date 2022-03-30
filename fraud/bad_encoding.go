package fraud

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/celestiaorg/rsmt2d"

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
			tree = NewErasuredNamespacedMerkleTree(uint64(len(errShares)))
			data = eds.Row(uint(index))
			if isRow {
				data = eds.Col(uint(index))
			}
			if err := buildTreeFromDataRoot(&tree, data); err != nil {
				continue
			}

			proof, err := tree.InclusionProof(uint8(0))
			if err != nil {
				return nil, err
			}
			shares[index] = &Share{errShares[index], &proof}
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
	if int(p.Position) >= len(header.DAH.RowsRoots) || int(p.Position) >= len(header.DAH.ColumnRoots) {
		return errors.New("invalid fraud proof: incorrect position of data square")
	}
	var rebuildNeeded bool
	root := header.DAH.RowsRoots
	if p.isRow {
		root = header.DAH.ColumnRoots
	}

	shares := make([][]byte, len(p.Shares))
	fmt.Println(len(p.Shares))
	// verify that Merkle proofs correspond to particular shares
	for index, share := range p.Shares {
		if share == nil {
			rebuildNeeded = true
			continue
		}
		shares[index] = share.Share
		if share.Proof == nil {
			continue
		}
		if ok := share.Validate(root[index]); !ok {
			return fmt.Errorf("invalid fraud proof: incorrect share received at position %d", index)
		}
	}

	if rebuildNeeded {
		var err error
		// rebuild a row/col
		codec := rsmt2d.NewRSGF8Codec()
		rebuildShares, err := codec.Decode(shares)
		if err != nil {
			return err
		}
		rebuiltExtendedShares, err := codec.Encode(rebuildShares)

		rebuildShares = append(
			rebuildShares,
			rebuiltExtendedShares...,
		)

		shares = rebuildShares
		fmt.Println(shares)
	}
	fmt.Println(shares)
	tree := NewErasuredNamespacedMerkleTree(uint64(len(shares)))
	for i, share := range shares {
		tree.Push(share, rsmt2d.SquareIndex{Axis: 0, Cell: uint(i)})
	}

	merkleRoot := header.DAH.ColumnRoots[p.Position]
	if p.isRow {
		merkleRoot = header.DAH.RowsRoots[p.Position]
	}
	// compare Merkle Roots
	if !bytes.Equal(tree.Root(), merkleRoot) {
		return errors.New("invalid fraud proof: merkle roots do not match")
	}
	return nil
}

func buildTreeFromDataRoot(tree *ErasuredNamespacedMerkleTree, root [][]byte) error {
	for idx, data := range root {
		if data == nil {
			return errors.New("empty share")
		}
		// error handling is not supposed here because ErasuredNamespacedMerkleTree panics in case of any error
		// TODO: should we rework this mechanism?
		tree.Push(data, rsmt2d.SquareIndex{Axis: uint(0), Cell: uint(idx)})

	}
	return nil
}
