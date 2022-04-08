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
	// ShareWithProof contains all shares from row or col.
	// Shares that did not pass verification in rmst2d will be nil.
	// For non-nil shares MerkleProofs are computed.
	Shares []*ShareWithProof
	// Index represents the row/col index where ErrByzantineRow/ErrByzantineColl occurred
	Index uint8
	// isRow shows that verification failed on row
	isRow bool
}

// CreateBadEncodingProof creates a new Bad Encoding Fraud Proof that should be propagated through network
// Fraud Proof will contain shares that did not pass the verification and appropriate to them Merkle Proofs
// In case if it's not possible to build Merkle Proof due to incomplete row/col,
// then missing shares should be get though sampling.
func CreateBadEncodingProof(
	ctx context.Context,
	height uint64,
	index uint8,
	isRow bool,
	eds *rsmt2d.ExtendedDataSquare,
	roots [][]byte,
	errShares [][]byte,
	dag format.NodeGetter,
) (Proof, error) {
	shares := make([]*ShareWithProof, len(errShares))
	for idx, share := range errShares {
		if share == nil {
			continue
		}
		// if error shares are from row, then Merkle Trees should genererated from
		// share respective column. It's supposed that columns are fully repaired. If not, then
		// missing shares should be get through sampling
		data := eds.Row(uint(idx))
		if isRow {
			// if error shares are from col, then Merkle Trees should genererated from
			// share respective row. It's supposed that columns are fully repaired. If not, then
			// missing shares should be get through sampling
			data = eds.Col(uint(idx))
		}

		tree, err := buildTreeFromLeaves(ctx, data, roots[idx], idx, dag)
		if err != nil {
			return nil, err
		}
		proof, err := tree.Tree().Prove(int(index))
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
		if idx >= len(errShares)/2 || int(index) >= len(errShares)/2 {
			namespaceID = consts.ParitySharesNamespaceID
		}

		shares[idx] = &ShareWithProof{namespaceID, share, &proof}
	}

	return &BadEncodingProof{
		BlockHeight: height,
		Shares:      shares,
		isRow:       isRow,
		Index:       index,
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

// MarshalBinary converts BadEncodingProof to binary
func (p *BadEncodingProof) MarshalBinary() ([]byte, error) {
	shares := make([]*pb.Share, 0, len(p.Shares))
	for _, share := range p.Shares {
		shares = append(shares, share.ShareWithProofToProto())
	}

	badEncodingFraudProof := pb.BadEnconding{
		Height: p.BlockHeight,
		Shares: shares,
		Index:  uint32(p.Index),
		IsRow:  p.isRow,
	}
	return badEncodingFraudProof.Marshal()
}

// UnmarshalBinary converts binary to BadEncodingProof
func (p *BadEncodingProof) UnmarshalBinary(data []byte) error {
	in := pb.BadEnconding{}
	if err := in.Unmarshal(data); err != nil {
		return err
	}
	befp := UnmarshalBEFP(&in)
	*p = *befp

	return nil
}

// UnmarshalBefp converts given data to BadEncodingProof
func UnmarshalBEFP(befp *pb.BadEnconding) *BadEncodingProof {
	return &BadEncodingProof{
		BlockHeight: befp.Height,
		Shares:      ProtoToShare(befp.Shares),
	}
}

// Validate ensures that Fraud Proof that was created by full node is correct.
// Validate checks that provided Merke Proofs are corresponded to particular shares,
// rebuilds bad row or col from received shares, computes Merke Root
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

	roots := merkleRowRoots
	// if error shares are from row, then Merkle Proofs should by verified against
	// respective column roots, because this Merkle Proofs were built against columns.
	// The logic is valid vice versa
	if p.isRow {
		roots = merkleColRoots
	}

	shares := make([][]byte, len(roots))

	// verify that Merkle proofs correspond to particular shares
	for index, share := range p.Shares {
		if share == nil {
			continue
		}
		shares[index] = share.Data
		if ok := share.Validate(roots[index]); !ok {
			return fmt.Errorf("invalid fraud proof: incorrect share received at Index %d", index)
		}
	}

	codec := consts.DefaultCodec()
	// rebuild a row or col
	rebuiltShares, err := codec.Decode(shares)
	if err != nil {
		return err
	}
	rebuiltExtendedShares, err := codec.Encode(rebuiltShares)
	if err != nil {
		return err
	}
	rebuiltShares = append(rebuiltShares, rebuiltExtendedShares...)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(roots) / 2))
	for i, share := range rebuiltShares {
		tree.Push(share, rsmt2d.SquareIndex{Axis: uint(p.Index), Cell: uint(i)})
	}

	// comparing rebuilt Merkle Root of bad row/col with respective Merkle Root of row/col from block
	merkleRoot := merkleColRoots[p.Index]
	if p.isRow {
		merkleRoot = merkleRowRoots[p.Index]
	}

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
		// Axis is an external shifting (e.g between Rows); Cell - is an internal shifting(e.g inside one col)
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
