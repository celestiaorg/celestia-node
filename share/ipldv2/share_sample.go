package ipldv2

import (
	"crypto/sha256"
	"errors"
	"fmt"

	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	nmtpb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	ipldv2pb "github.com/celestiaorg/celestia-node/share/ipldv2/pb"
)

// ShareSampleType represents type of sample.
type ShareSampleType uint8

const (
	// DataShareSample is a sample of a data share.
	DataShareSample ShareSampleType = iota
	// ParityShareSample is a sample of a parity share.
	ParityShareSample
)

// ShareSample represents a sample of an NMT in EDS.
type ShareSample struct {
	// ID of the ShareSample
	ID ShareSampleID
	// Type of the ShareSample
	Type ShareSampleType
	// Proof of Share inclusion in the NMT
	Proof nmt.Proof
	// Share being sampled
	Share share.Share
}

// NewShareSample constructs a new ShareSample.
func NewShareSample(id ShareSampleID, shr share.Share, proof nmt.Proof, sqrLn int) *ShareSample {
	row, col := id.Index/sqrLn, id.Index%sqrLn
	tp := ParityShareSample
	if row < sqrLn/2 && col < sqrLn/2 {
		tp = DataShareSample
	}

	return &ShareSample{
		ID:    id,
		Type:  tp,
		Proof: proof,
		Share: shr,
	}
}

// NewShareSampleFrom constructs a new ShareSample from share.Root.
func NewShareSampleFrom(root *share.Root, idx int, axis rsmt2d.Axis, shr share.Share, proof nmt.Proof) *ShareSample {
	id := NewShareSampleID(root, idx, axis)
	return NewShareSample(id, shr, proof, len(root.RowRoots))
}

// NewShareSampleFromEDS samples the EDS and constructs a new ShareSample.
func NewShareSampleFromEDS(eds *rsmt2d.ExtendedDataSquare, idx int, axis rsmt2d.Axis) (*ShareSample, error) {
	sqrLn := int(eds.Width())
	axisIdx, shrIdx := idx/sqrLn, idx%sqrLn

	// TODO(@Wondertan): Should be an rsmt2d method
	var shrs [][]byte
	switch axis {
	case rsmt2d.Row:
		shrs = eds.Row(uint(axisIdx))
	case rsmt2d.Col:
		axisIdx, shrIdx = shrIdx, axisIdx
		shrs = eds.Col(uint(axisIdx))
	default:
		panic("invalid axis")
	}

	root, err := share.NewRoot(eds)
	if err != nil {
		return nil, fmt.Errorf("while computing root: %w", err)
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(sqrLn/2), uint(axisIdx))
	for _, shr := range shrs {
		err := tree.Push(shr)
		if err != nil {
			return nil, fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	prf, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, fmt.Errorf("while proving range share over NMT: %w", err)
	}

	return NewShareSampleFrom(root, idx, axis, shrs[shrIdx], prf), nil
}

// Proto converts ShareSample to its protobuf representation.
func (s *ShareSample) Proto() *ipldv2pb.ShareSample {
	// TODO: Extract as helper to nmt
	proof := &nmtpb.Proof{}
	proof.Nodes = s.Proof.Nodes()
	proof.End = int64(s.Proof.End())
	proof.Start = int64(s.Proof.Start())
	proof.IsMaxNamespaceIgnored = s.Proof.IsMaxNamespaceIDIgnored()
	proof.LeafHash = s.Proof.LeafHash()

	return &ipldv2pb.ShareSample{
		Id:    s.ID.Proto(),
		Type:  ipldv2pb.ShareSampleType(s.Type),
		Proof: proof,
		Share: s.Share,
	}
}

// ShareSampleFromBlock converts blocks.Block into ShareSample.
func ShareSampleFromBlock(blk blocks.Block) (*ShareSample, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}

	s := &ShareSample{}
	err := s.UnmarshalBinary(blk.RawData())
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling ShareSample: %w", err)
	}

	return s, nil
}

// IPLDBlock converts ShareSample to an IPLD block for Bitswap compatibility.
func (s *ShareSample) IPLDBlock() (blocks.Block, error) {
	cid, err := s.ID.Cid()
	if err != nil {
		return nil, err
	}

	data, err := s.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, cid)
}

// MarshalBinary marshals ShareSample to binary.
func (s *ShareSample) MarshalBinary() ([]byte, error) {
	return s.Proto().Marshal()
}

// UnmarshalBinary unmarshal ShareSample from binary.
func (s *ShareSample) UnmarshalBinary(data []byte) error {
	proto := &ipldv2pb.ShareSample{}
	if err := proto.Unmarshal(data); err != nil {
		return err
	}

	s.ID = ShareSampleID{
		DataHash: proto.Id.DataHash,
		AxisHash: proto.Id.AxisHash,
		Index:    int(proto.Id.Index),
		Axis:     rsmt2d.Axis(proto.Id.Axis),
	}
	s.Type = ShareSampleType(proto.Type)
	s.Proof = nmt.ProtoToProof(*proto.Proof)
	s.Share = proto.Share
	return nil
}

// Validate validates ShareSample's fields and proof of Share inclusion in the NMT.
func (s *ShareSample) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}

	if s.Type != DataShareSample && s.Type != ParityShareSample {
		return fmt.Errorf("incorrect sample type: %d", s.Type)
	}

	namespace := share.ParitySharesNamespace
	if s.Type == DataShareSample {
		namespace = share.GetNamespace(s.Share)
	}

	if !s.Proof.VerifyInclusion(sha256.New(), namespace.ToNMT(), [][]byte{s.Share}, s.ID.AxisHash) {
		return errors.New("sample proof is invalid")
	}

	return nil
}
