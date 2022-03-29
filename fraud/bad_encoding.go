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
	fmt.Println(len(errShares))
	shares := make([]*Share, len(errShares))
	//OUTER:
	for index, share := range errShares {
		if share != nil {
			tree = NewErasuredNamespacedMerkleTree(uint64(len(errShares)))
			data := eds.Row(uint(index))
			if isRow {
				data = eds.Col(uint(index))
			}
			for i, value := range data {
				tree.Push(value, rsmt2d.SquareIndex{Axis: uint(position), Cell: uint(i)})
				if tree.Err != nil {
					fmt.Println(tree.Err)
					//continue OUTER
				}
			}

			proof, err := tree.InclusionProof(uint8(position))
			if err != nil {
				return nil, err
			}
			d := tree.PrepareData(rsmt2d.SquareIndex{Axis: uint(position), Cell: uint(0)}, errShares[index])
			shares[index] = &Share{d, proof}
			fmt.Println(shares[index].Validate(eds.ColRoots()[index]))
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

	root := header.DAH.ColumnRoots[p.Position]
	if p.isRow {
		root = header.DAH.RowsRoots[p.Position]
	}

	shares := make([][]byte, len(p.Shares))
	// verify that Merkle proofs correspond to particular shares
	for index, share := range p.Shares {
		if share != nil {
			if ok := share.Validate(root); !ok {
				return fmt.Errorf("invalid fraud proof: incorrect share received at position %d", index)
			}
			shares[index] = share.Share
		}
	}

	// // rebuilt a row/col
	codec := rsmt2d.NewRSGF8Codec()
	rebuiltShares, err := codec.Encode(shares)
	if err != nil {
		return err
	}

	tree := NewErasuredNamespacedMerkleTree(uint64(len(rebuiltShares)))
	for i, share := range rebuiltShares {
		tree.Push(share, rsmt2d.SquareIndex{Axis: 0, Cell: uint(i)})
	}

	// compare Merkle Roots
	if !bytes.Equal(tree.Root(), root) {
		return errors.New("invalid fraud proof: merkle roots do not match")
	}
	return nil
}
