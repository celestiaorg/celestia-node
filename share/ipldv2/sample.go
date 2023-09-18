package ipldv2

import (
	"crypto/sha256"
	"errors"

	"github.com/celestiaorg/nmt"
	nmtpb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"
	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-node/share"
	ipldv2pb "github.com/celestiaorg/celestia-node/share/ipldv2/pb"
)

type Sample struct {
	ID    SampleID
	Type  SampleType
	Proof nmt.Proof
	Share share.Share
}

type SampleType uint8

const (
	DataSample SampleType = iota
	ParitySample
)

func NewSample(root *share.Root, idx int, axis rsmt2d.Axis, shr share.Share, proof nmt.Proof) *Sample {
	id := NewSampleID(root, idx, axis)

	sqrLn := len(root.RowRoots)
	row, col := idx/sqrLn, idx%sqrLn
	tp := ParitySample
	if row < sqrLn/2 && col < sqrLn/2 {
		tp = DataSample
	}

	return &Sample{
		ID:    id,
		Type:  tp,
		Proof: proof,
		Share: shr,
	}
}

func (s *Sample) Proto() *ipldv2pb.Sample {
	// TODO: Extract as helper to nmt
	proof := &nmtpb.Proof{}
	proof.Nodes = s.Proof.Nodes()
	proof.End = int64(s.Proof.End())
	proof.Start = int64(s.Proof.Start())
	proof.IsMaxNamespaceIgnored = s.Proof.IsMaxNamespaceIDIgnored()
	proof.LeafHash = s.Proof.LeafHash()

	return &ipldv2pb.Sample{
		Id:    s.ID.Proto(),
		Type:  ipldv2pb.SampleType(s.Type),
		Proof: proof,
		Share: s.Share,
	}
}

func (s *Sample) IPLDBlock() (blocks.Block, error) {
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

func (s *Sample) MarshalBinary() ([]byte, error) {
	return s.Proto().Marshal()
}

func (s *Sample) UnmarshalBinary(data []byte) error {
	proto := &ipldv2pb.Sample{}
	err := proto.Unmarshal(data)
	if err != nil {
		return err
	}

	s.ID = SampleID{
		DataRoot: proto.Id.DataRoot,
		DAHRoot:  proto.Id.DahRoot,
		Index:    int(proto.Id.Index),
		Axis:     rsmt2d.Axis(proto.Id.Axis),
	}
	s.Type = SampleType(proto.Type)
	s.Proof = nmt.ProtoToProof(*proto.Proof)
	s.Share = proto.Share
	return nil
}

func (s *Sample) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}

	if s.Type != DataSample && s.Type != ParitySample {
		return errors.New("malformed sample type")
	}

	// TODO Support Col proofs
	namespace := share.ParitySharesNamespace
	if s.Type == DataSample {
		namespace = share.GetNamespace(s.Share)
	}

	if !s.Proof.VerifyInclusion(sha256.New(), namespace.ToNMT(), [][]byte{s.Share}, s.ID.DAHRoot) {
		return errors.New("sample proof is invalid")
	}

	return nil
}
