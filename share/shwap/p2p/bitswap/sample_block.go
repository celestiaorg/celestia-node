package bitswap

import (
	"context"
	"fmt"
	"math"

	"github.com/ipfs/go-cid"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

const (
	// sampleCodec is a CID codec used for share sampling Bitswap requests over Namespaced
	// Merkle Tree.
	sampleCodec = 0x7810

	// sampleMultihashCode is the multihash code for share sampling multihash function.
	sampleMultihashCode = 0x7811
)

// maxSampleSize is the maximum size of the SampleBlock.
// It is calculated as the size of the share plus the size of the proof.
var maxSampleSize = libshare.ShareSize + share.AxisRootSize*int(math.Log2(float64(share.MaxSquareSize)))

func init() {
	registerBlock(
		sampleMultihashCode,
		sampleCodec,
		maxSampleSize,
		shwap.SampleIDSize,
		func(cid cid.Cid) (Block, error) {
			return EmptySampleBlockFromCID(cid)
		},
	)
}

// SampleBlock is a Bitswap compatible block for Shwap's Sample container.
type SampleBlock struct {
	ID        shwap.SampleID
	Container shwap.Sample
}

// NewEmptySampleBlock constructs a new empty SampleBlock.
func NewEmptySampleBlock(height uint64, idx shwap.SampleCoords, edsSize int) (*SampleBlock, error) {
	id, err := shwap.NewSampleID(height, idx, edsSize)
	if err != nil {
		return nil, err
	}

	return &SampleBlock{ID: id}, nil
}

// EmptySampleBlockFromCID constructs an empty SampleBlock out of the CID.
func EmptySampleBlockFromCID(cid cid.Cid) (*SampleBlock, error) {
	sidData, err := extractFromCID(cid)
	if err != nil {
		return nil, err
	}

	sid, err := shwap.SampleIDFromBinary(sidData)
	if err != nil {
		return nil, fmt.Errorf("while unmarhaling SampleBlock: %w", err)
	}

	return &SampleBlock{ID: sid}, nil
}

func (sb *SampleBlock) CID() cid.Cid {
	return encodeToCID(sb.ID, sampleMultihashCode, sampleCodec)
}

func (sb *SampleBlock) Height() uint64 {
	return sb.ID.Height()
}

func (sb *SampleBlock) Marshal() ([]byte, error) {
	if sb.Container.IsEmpty() {
		return nil, fmt.Errorf("cannot marshal empty SampleBlock")
	}

	container := sb.Container.ToProto()
	containerData, err := container.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling SampleBlock container: %w", err)
	}

	return containerData, nil
}

func (sb *SampleBlock) Populate(ctx context.Context, eds eds.Accessor) error {
	idx := shwap.SampleCoords{Row: sb.ID.RowIndex, Col: sb.ID.ShareIndex}

	smpl, err := eds.Sample(ctx, idx)
	if err != nil {
		return fmt.Errorf("accessing Sample: %w", err)
	}

	sb.Container = smpl
	return nil
}

func (sb *SampleBlock) UnmarshalFn(root *share.AxisRoots) UnmarshalFn {
	return func(cntrData, idData []byte) error {
		if !sb.Container.IsEmpty() {
			return nil
		}

		sid, err := shwap.SampleIDFromBinary(idData)
		if err != nil {
			return fmt.Errorf("unmarhaling SampleID: %w", err)
		}

		if !sb.ID.Equals(sid) {
			return fmt.Errorf("requested %+v doesnt match given %+v", sb.ID, sid)
		}

		var sample shwappb.Sample
		if err := sample.Unmarshal(cntrData); err != nil {
			return fmt.Errorf("unmarshaling Sample for %+v: %w", sb.ID, err)
		}

		cntr, err := shwap.SampleFromProto(&sample)
		if err != nil {
			return fmt.Errorf("unmarshaling Sample for %+v: %w", sb.ID, err)
		}

		if err = cntr.Verify(root, sb.ID.RowIndex, sb.ID.ShareIndex); err != nil {
			return fmt.Errorf("validating Sample for %+v: %w", sb.ID, err)
		}

		sb.Container = cntr
		return nil
	}
}
