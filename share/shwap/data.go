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

	DataProof  nmt.Proof
	DataShares []share.Share
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

		shrs := square.Row(uint(rowIdx))
		// TDOD(@Wondertan): This will likely be removed
		nd, proof, err := eds.NDFromShares(shrs, namespace, rowIdx)
		if err != nil {
			return nil, err
		}

		id := NewDataID(rowIdx, root, height, namespace)
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
	cid, err := s.DataID.Cid()
	if err != nil {
		return nil, err
	}

	data, err := s.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, cid)
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
		DataProof:  proof,
		DataShares: s.DataShares,
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

	s.DataProof = nmt.ProtoToProof(*proto.DataProof)
	s.DataShares = proto.DataShares
	return nil
}

// Validate performs basic validation of Data.
func (s *Data) Validate() error {
	if err := s.DataID.Validate(); err != nil {
		return err
	}

	if len(s.DataShares) == 0 {
		return fmt.Errorf("empty DataShares")
	}

	shrs := make([][]byte, 0, len(s.DataShares))
	for _, shr := range s.DataShares {
		shrs = append(shrs, append(share.GetNamespace(shr), shr...))
	}

	s.DataProof.WithHashedProof(hasher())
	if !s.DataProof.VerifyNamespace(hasher(), s.DataNamespace.ToNMT(), shrs, s.AxisHash) {
		return fmt.Errorf("invalid DataProof")
	}

	return nil
}
