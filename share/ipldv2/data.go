package ipldv2

import (
	"fmt"

	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	ipldv2pb "github.com/celestiaorg/celestia-node/share/ipldv2/pb"
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

// // NewDataFromEDS samples the EDS and constructs a new Data.
// func NewDataFromEDS(
// 	idx int,
// 	eds *rsmt2d.ExtendedDataSquare,
// 	height uint64,
// ) (*Data, error) {
// 	sqrLn := int(eds.Width())
//
// 	// TODO(@Wondertan): Should be an rsmt2d method
// 	var axisHalf [][]byte
// 	switch axisType {
// 	case rsmt2d.Row:
// 		axisHalf = eds.Row(uint(idx))[:sqrLn/2]
// 	case rsmt2d.Col:
// 		axisHalf = eds.Col(uint(idx))[:sqrLn/2]
// 	default:
// 		panic("invalid axis")
// 	}
//
// 	root, err := share.NewRoot(eds)
// 	if err != nil {
// 		return nil, fmt.Errorf("while computing root: %w", err)
// 	}
//
// 	id := NewDataID(axisType, uint16(idx), root, height)
// 	return NewData(id, axisHalf), nil
// }

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

	return (&ipldv2pb.Data{
		DataId:     id,
		DataShares: s.DataShares,
	}).Marshal()
}

// UnmarshalBinary unmarshal Data from binary.
func (s *Data) UnmarshalBinary(data []byte) error {
	proto := &ipldv2pb.Data{}
	if err := proto.Unmarshal(data); err != nil {
		return err
	}

	err := s.DataID.UnmarshalBinary(proto.DataId)
	if err != nil {
		return err
	}

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

	s.DataProof.WithHashedProof(hasher())
	if !s.DataProof.VerifyNamespace(hasher(), s.DataNamespace.ToNMT(), s.DataShares, s.AxisHash) {
		return fmt.Errorf("invalid DataProof")
	}

	return nil
}
