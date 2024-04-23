package shwap

import (
	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-node/share"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

type Data struct {
	DataID
	share.RowNamespaceData
}

// NewData constructs a new Data.
func NewData(id DataID, nr share.RowNamespaceData) *Data {
	return &Data{
		DataID:           id,
		RowNamespaceData: nr,
	}
}

// DataFromBlock converts blocks.Block into Data.
func DataFromBlock(blk blocks.Block) (*Data, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}

	var data shwappb.RowNamespaceDataBlock
	if err := data.Unmarshal(blk.RawData()); err != nil {
		return nil, err

	}
	return DataFromProto(&data)
}

// IPLDBlock converts Data to an IPLD block for Bitswap compatibility.
func (s *Data) IPLDBlock() (blocks.Block, error) {
	data, err := s.ToProto().Marshal()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, s.Cid())
}

// Verify validates Data's fields and verifies Data inclusion.
func (s *Data) Verify(root *share.Root) error {
	if err := s.DataID.Verify(root); err != nil {
		return err
	}

	return s.RowNamespaceData.Validate(root, int(s.DataID.RowIndex), s.DataID.Namespace())
}

func (s *Data) ToProto() *shwappb.RowNamespaceDataBlock {
	return &shwappb.RowNamespaceDataBlock{
		DataId: s.DataID.MarshalBinary(),
		Data:   s.RowNamespaceData.ToProto(),
	}
}

func DataFromProto(dataProto *shwappb.RowNamespaceDataBlock) (*Data, error) {
	id, err := DataIDFromBinary(dataProto.DataId)
	if err != nil {
		return nil, err
	}
	return &Data{
		DataID:           id,
		RowNamespaceData: share.NamespacedRowFromProto(dataProto.Data),
	}, nil
}
