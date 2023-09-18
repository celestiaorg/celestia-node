package ipldv2

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/celestiaorg/celestia-node/share"
	ipldv2pb "github.com/celestiaorg/celestia-node/share/ipldv2/pb"
	"github.com/celestiaorg/rsmt2d"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// SampleIDSize is the size of the SampleID in bytes
const SampleIDSize = 127

// TODO: Harden with validation of fields and inputs
type SampleID struct {
	DataRoot []byte
	DAHRoot  []byte
	Index    int
	Axis     rsmt2d.Axis
}

func NewSampleID(root *share.Root, idx int, axis rsmt2d.Axis) SampleID {
	sqrLn := len(root.RowRoots)
	row, col := idx/sqrLn, idx%sqrLn
	dahroot := root.RowRoots[row]
	if axis == rsmt2d.Col {
		dahroot = root.ColumnRoots[col]
	}

	return SampleID{
		DataRoot: root.Hash(),
		DAHRoot:  dahroot,
		Index:    idx,
		Axis:     axis,
	}
}

func (s *SampleID) Cid() (cid.Cid, error) {
	data, err := s.MarshalBinary()
	if err != nil {
		return cid.Undef, err
	}

	buf, err := mh.Encode(data, multihashCode)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(codec, buf), nil
}

func (s *SampleID) Proto() *ipldv2pb.SampleID {
	return &ipldv2pb.SampleID{
		DataRoot: s.DataRoot,
		DahRoot:  s.DAHRoot,
		Index:    uint32(s.Index),
		Axis:     ipldv2pb.Axis(s.Axis),
	}
}

func (s *SampleID) MarshalBinary() ([]byte, error) {
	// we cannot use protobuf here because it exceeds multihash limit of 128 bytes
	data := make([]byte, 127)
	n := copy(data, s.DataRoot)
	n += copy(data[n:], s.DAHRoot)
	binary.LittleEndian.PutUint32(data[n:], uint32(s.Index))
	data[n+4] = byte(s.Axis)
	return data, nil
}

// TODO(@Wondertan): Eventually this should become configurable
const (
	hashSize = sha256.Size
	dahRootSize = 2*share.NamespaceSize + hashSize
)

func (s *SampleID) Validate() error {
	if len(s.DataRoot) != hashSize {
		return errors.New("malformed data root")
	}

	if len(s.DAHRoot) != dahRootSize {
		return errors.New("malformed DAH root")
	}

	if s.Axis != rsmt2d.Col && s.Axis != rsmt2d.Row {
		return errors.New("malformed axis")
	}

	return nil
}
