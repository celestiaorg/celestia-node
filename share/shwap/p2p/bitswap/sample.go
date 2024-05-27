package bitswap

import (
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	bitswappb "github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap/pb"
)

const (
	// sampleCodec is a CID codec used for share sampling Bitswap requests over Namespaced
	// Merkle Tree.
	sampleCodec = 0x7810

	// sampleMultihashCode is the multihash code for share sampling multihash function.
	sampleMultihashCode = 0x7811
)

func init() {
	RegisterID(
		sampleMultihashCode,
		sampleCodec,
		shwap.SampleIDSize,
		func(cid cid.Cid) (blockBuilder, error) {
			return SampleIDFromCID(cid)
		},
	)
}

type SampleID shwap.SampleID

// SampleIDFromCID coverts CID to SampleID.
func SampleIDFromCID(cid cid.Cid) (SampleID, error) {
	sidData, err := extractCID(cid)
	if err != nil {
		return SampleID{}, err
	}

	sid, err := shwap.SampleIDFromBinary(sidData)
	if err != nil {
		return SampleID{}, fmt.Errorf("while unmarhaling SampleID: %w", err)
	}

	return SampleID(sid), nil
}

func (sid SampleID) String() string {
	data, err := sid.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling SampleID: %w", err))
	}
	return string(data)
}

func (sid SampleID) MarshalBinary() ([]byte, error) {
	return shwap.SampleID(sid).MarshalBinary()
}

func (sid SampleID) CID() cid.Cid {
	return encodeCID(shwap.SampleID(sid), sampleMultihashCode, sampleCodec)
}

func (sid SampleID) BlockFromEDS(eds *rsmt2d.ExtendedDataSquare) (blocks.Block, error) {
	smpl, err := shwap.SampleFromEDS(eds, rsmt2d.Row, sid.RowIndex, sid.ShareIndex)
	if err != nil {
		return nil, err
	}

	dataID, err := sid.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshaling SampleID: %w", err)
	}

	smplBlk := bitswappb.SampleBlock{
		SampleId: dataID,
		Sample:   smpl.ToProto(),
	}

	dataBlk, err := smplBlk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling SampleBlock: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(dataBlk, sid.CID())
	if err != nil {
		return nil, fmt.Errorf("assembling block: %w", err)
	}

	return blk, nil
}

func (sid SampleID) UnmarshalContainer(root *share.Root, data []byte) (shwap.Sample, error) {
	var sampleBlk bitswappb.SampleBlock
	if err := sampleBlk.Unmarshal(data); err != nil {
		return shwap.Sample{}, fmt.Errorf("unmarshaling SampleBlock: %w", err)
	}

	sample := shwap.SampleFromProto(sampleBlk.Sample)
	if err := sample.Validate(root, sid.RowIndex, sid.ShareIndex); err != nil {
		return shwap.Sample{}, fmt.Errorf("validating Sample: %w", err)
	}
	// NOTE: We don't have to validate ID in the RowBlock, as it's implicitly verified by string
	// equality of globalVerifiers entry key(requesting side) and hasher accessing the entry(response
	// verification)
	return sample, nil
}
