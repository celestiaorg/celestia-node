package ipldv2

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

const (
	// codec is the codec used for leaf and inner nodes of a Namespaced Merkle Tree.
	codec = 0x7800

	// multihashCode is the multihash code used to hash blocks
	// that contain an NMT node (inner and leaf nodes).
	multihashCode = 0x7801

	nmtHashSize = 2*share.NamespaceSize + sha256.Size
)

// ShareCID returns the CID of the share with the given index in the given dataroot.
func ShareCID(root *share.Root, idx int, axis rsmt2d.Axis) (cid.Cid, error) {
	if idx < 0 || idx >= len(root.ColumnRoots) {
		return cid.Undef, fmt.Errorf("invalid share index")
	}

	dataroot := root.Hash()
	axisroot := root.RowRoots[axis]
	if axis == rsmt2d.Col {
		axisroot = root.ColumnRoots[axis]
	}

	data := make([]byte, sha256.Size+nmtHashSize+4)
	n := copy(data, dataroot)
	n += copy(data[n:], axisroot)
	binary.LittleEndian.PutUint32(data[n:], uint32(idx))

	buf, err := mh.Encode(data, multihashCode)
	if err != nil {
		return cid.Undef, err
	}
	return cid.NewCidV1(codec, buf), nil
}

type Hasher struct {
	data []byte
}

func (h *Hasher) Write(data []byte) (int, error) {
	if h.data != nil {
		panic("only a single Write is allowed")
	}
	// TODO Check size
	// TODO Support Col proofs

	axisroot := data[sha256.Size : sha256.Size+nmtHashSize]
	shareData := data[sha256.Size+nmtHashSize+8 : sha256.Size+nmtHashSize+8+share.Size]
	proofData := data[sha256.Size+nmtHashSize+8+share.Size:]

	proofPb := pb.Proof{}
	err := proofPb.Unmarshal(proofData)
	if err != nil {
		return 0, err
	}

	proof := nmt.ProtoToProof(proofPb)
	if proof.VerifyInclusion(sha256.New(), share.GetNamespace(shareData).ToNMT(), [][]byte{shareData}, axisroot) {
		return len(data), nil
	}

	h.data = data
	return len(h.data), nil
}

func (h *Hasher) Sum([]byte) []byte {
	return h.data[:sha256.Size+nmtHashSize+4]
}

// Reset resets the Hash to its initial state.
func (h *Hasher) Reset() {
	h.data = nil
}

func (h *Hasher) Size() int {
	return sha256.Size + nmtHashSize + 4
}

// BlockSize returns the hash's underlying block size.
func (h *Hasher) BlockSize() int {
	return sha256.BlockSize
}

// NewBlockservice constructs Blockservice for fetching NMTrees.
func NewBlockservice(bs blockstore.Blockstore, exchange exchange.Interface) blockservice.BlockService {
	return blockservice.New(bs, exchange, blockservice.WithAllowlist(defaultAllowlist))
}

// NewMemBlockservice constructs Blockservice for fetching NMTrees with in-memory blockstore.
func NewMemBlockservice() blockservice.BlockService {
	bstore := blockstore.NewBlockstore(sync.MutexWrap(datastore.NewMapDatastore()))
	return NewBlockservice(bstore, nil)
}

// defaultAllowlist keeps default list of hashes allowed in the network.
var defaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we disable all codes except home-baked code
	return code == multihashCode
}

type node struct {
	data []byte
}

func MakeNode(root *share.Root, axis rsmt2d.Axis, idx int, shr share.Share, proof *nmt.Proof) (block.Block, error) {
	dataroot := root.Hash()
	axisroot := root.RowRoots[axis]
	if axis == rsmt2d.Col {
		axisroot = root.ColumnRoots[axis]
	}

	var data []byte
	data = append(data, dataroot...)
	data = append(data, axisroot...)
	data = binary.LittleEndian.AppendUint32(data, uint32(idx))
	data = append(data, shr...)

	proto := pb.Proof{}
	proto.Nodes = proof.Nodes()
	proto.End = int64(proof.End())
	proto.Start = int64(proof.Start())
	proto.IsMaxNamespaceIgnored = proof.IsMaxNamespaceIDIgnored()
	proto.LeafHash = proof.LeafHash()

	proofData, err := proto.Marshal()
	if err != nil {
		return nil, err
	}

	data = append(data, proofData...)
	return &node{
		data: data,
	}, nil
}

func (n *node) RawData() []byte {
	return n.data
}

func (n *node) Cid() cid.Cid {
	buf, err := mh.Encode(n.data[:sha256.Size+nmtHashSize+4], multihashCode)
	if err != nil {
		panic(err)
	}

	return cid.NewCidV1(codec, buf)
}

func (n *node) String() string {
	// TODO implement me
	panic("implement me")
}

func (n *node) Loggable() map[string]interface{} {
	// TODO implement me
	panic("implement me")
}
