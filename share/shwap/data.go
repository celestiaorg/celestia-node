package shwap

import (
	"fmt"

	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/nmt"
	nmtpb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

type Data struct {
	DataID

	DataShares []share.Share
	DataProof  nmt.Proof
}

// NewData constructs a new Data.
func NewData(id DataID, shares []share.Share, proof nmt.Proof) *Data {
	return &Data{
		DataID:     id,
		DataShares: shares,
		DataProof:  proof,
	}
}

// NewDataFromEDS samples the EDS and constructs Data for each row with the given namespace.
func NewDataFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	height uint64,
	namespace share.Namespace,
) ([]*Data, error) {
	root, err := share.NewRoot(square)
	if err != nil {
		return nil, fmt.Errorf("while computing root: %w", err)
	}

	var datas []*Data //nolint:prealloc// we don't know how many rows with needed namespace there are
	for rowIdx, rowRoot := range root.RowRoots {
		if namespace.IsOutsideRange(rowRoot, rowRoot) {
			continue
		}

		id, err := NewDataID(height, uint16(rowIdx), namespace, root)
		if err != nil {
			return nil, err
		}

		shrs := square.Row(uint(rowIdx))
		// TDOD(@Wondertan): This will likely be removed
		nd, proof, err := eds.NDFromShares(shrs, namespace, rowIdx)
		if err != nil {
			return nil, err
		}

		datas = append(datas, NewData(id, nd, proof))
	}

	return datas, nil
}

// DataFromBlock converts blocks.Block into Data.
func DataFromBlock(blk blocks.Block) (*Data, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}

	s := &Data{}
	err := s.UnmarshalBinary(blk.RawData())
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling Data: %w", err)
	}

	return s, nil
}

// IPLDBlock converts Data to an IPLD block for Bitswap compatibility.
func (s *Data) IPLDBlock() (blocks.Block, error) {
	data, err := s.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, s.Cid())
}

// MarshalBinary marshals Data to binary.
func (s *Data) MarshalBinary() ([]byte, error) {
	id, err := s.DataID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	proof := &nmtpb.Proof{}
	proof.Nodes = s.DataProof.Nodes()
	proof.End = int64(s.DataProof.End())
	proof.Start = int64(s.DataProof.Start())
	proof.IsMaxNamespaceIgnored = s.DataProof.IsMaxNamespaceIDIgnored()
	proof.LeafHash = s.DataProof.LeafHash()

	return (&shwappb.Data{
		DataId:     id,
		DataShares: s.DataShares,
		DataProof:  proof,
	}).Marshal()
}

// UnmarshalBinary unmarshal Data from binary.
func (s *Data) UnmarshalBinary(data []byte) error {
	proto := &shwappb.Data{}
	if err := proto.Unmarshal(data); err != nil {
		return err
	}

	err := s.DataID.UnmarshalBinary(proto.DataId)
	if err != nil {
		return err
	}

	s.DataShares = proto.DataShares
	s.DataProof = nmt.ProtoToProof(*proto.DataProof)
	return nil
}

// Verify validates Data's fields and verifies Data inclusion.
func (s *Data) Verify(root *share.Root) error {
	if err := s.DataID.Verify(root); err != nil {
		return err
	}

	if len(s.DataShares) == 0 && s.DataProof.IsEmptyProof() {
		return fmt.Errorf("empty Data")
	}

	shrs := make([][]byte, 0, len(s.DataShares))
	for _, shr := range s.DataShares {
		shrs = append(shrs, append(share.GetNamespace(shr), shr...))
	}

	rowRoot := root.RowRoots[s.RowIndex]
	if !s.DataProof.VerifyNamespace(hashFn(), s.Namespace().ToNMT(), shrs, rowRoot) {
		return fmt.Errorf("invalid DataProof")
	}

	return nil
}
