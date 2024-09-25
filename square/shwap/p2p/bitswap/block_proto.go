package bitswap

import (
	"fmt"

	"github.com/ipfs/go-cid"

	bitswappb "github.com/celestiaorg/celestia-node/square/shwap/p2p/bitswap/pb"
)

// marshalProto wraps the given Block in composition protobuf and marshals it.
func marshalProto(blk Block) ([]byte, error) {
	containerData, err := blk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling Shwap container: %w", err)
	}

	blkProto := bitswappb.Block{
		Cid:       blk.CID().Bytes(),
		Container: containerData,
	}

	blkData, err := blkProto.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling Bitswap Block protobuf: %w", err)
	}

	return blkData, nil
}

// unmarshalProto unwraps given data from composition protobuf and provides
// inner CID and serialized container data.
func unmarshalProto(data []byte) (cid.Cid, []byte, error) {
	var blk bitswappb.Block
	err := blk.Unmarshal(data)
	if err != nil {
		return cid.Undef, nil, fmt.Errorf("unmarshalling protobuf block: %w", err)
	}

	cid, err := cid.Cast(blk.Cid)
	if err != nil {
		return cid, nil, fmt.Errorf("casting cid: %w", err)
	}

	return cid, blk.Container, nil
}
