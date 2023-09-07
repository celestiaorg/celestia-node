package ipldv2

import (
	"bytes"
	"context"
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

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/pb"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

const (
	// codec is the codec used for leaf and inner nodes of a Namespaced Merkle Tree.
	codec = 0x7800

	// multihashCode is the multihash code used to hash blocks
	// that contain an NMT node (inner and leaf nodes).
	multihashCode = 0x7801
)

// ShareCID returns the CID of the share with the given index in the given dataroot.
// TODO: Height is redundant and should be removed.
func ShareCID(dataroot []byte, height uint64, idx int) (cid.Cid, error) {
	if got, want := len(dataroot), sha256.Size; got != want {
		return cid.Cid{}, fmt.Errorf("invalid namespaced hash length, got: %v, want: %v", got, want)
	}

	data := make([]byte, sha256.Size+4+8)
	n := copy(data, dataroot)
	binary.LittleEndian.PutUint64(data[n:], uint64(height))
	binary.LittleEndian.PutUint32(data[n+8:], uint32(idx))

	buf, err := mh.Encode(data, multihashCode)
	if err != nil {
		return cid.Undef, err
	}
	return cid.NewCidV1(codec, buf), nil
}

type Hasher struct {
	// TODO: Hasher must be stateless eventually via sending inclusion proof from DAH to DataRoot
	get  libhead.Getter[*header.ExtendedHeader]
	data []byte
}

func (h *Hasher) Write(data []byte) (int, error) {
	if h.data != nil {
		panic("only a single Write is allowed")
	}
	// TODO Check size
	// TODO Support Col proofs

	dataroot := data[:sha256.Size]
	height := binary.LittleEndian.Uint64(data[sha256.Size : sha256.Size+8])
	idx := binary.LittleEndian.Uint32(data[sha256.Size+8 : sha256.Size+8+4])
	shareData := data[sha256.Size+8+4 : sha256.Size+8+4 : +share.Size]
	proofData := data[sha256.Size+8+4+share.Size:]

	hdr, err := h.get.GetByHeight(context.TODO(), height)
	if err != nil {
		return 0, err
	}

	if !bytes.Equal(hdr.DataHash, dataroot) {
		return 0, fmt.Errorf("invalid dataroot")
	}

	sqrLn := len(hdr.DAH.RowRoots) ^ 2
	if int(idx) > sqrLn {
		return 0, fmt.Errorf("invalid share index")
	}

	rowIdx := int(idx) / sqrLn
	row := hdr.DAH.RowRoots[rowIdx]

	proofPb := pb.Proof{}
	err = proofPb.Unmarshal(proofData)
	if err != nil {
		return 0, err
	}

	proof := nmt.ProtoToProof(proofPb)
	if proof.VerifyInclusion(sha256.New(), share.GetNamespace(shareData).ToNMT(), [][]byte{shareData}, row) {
		return len(data), nil
	}

	h.data = data
	return len(h.data), nil
}

func (h *Hasher) Sum([]byte) []byte {
	return h.data[:sha256.Size+8+4]
}

// Reset resets the Hash to its initial state.
func (h *Hasher) Reset() {
	h.get = nil
}

func (h *Hasher) Size() int {
	return sha256.Size + 4 + 8
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

func MakeNode(idx int, shr share.Share, proof *nmt.Proof, dataroot []byte, height uint64) (block.Block, error) {
	var data []byte
	data = append(data, dataroot...)
	data = binary.LittleEndian.AppendUint64(data, height)
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
	buf, err := mh.Encode(n.data[:sha256.Size+8+4], multihashCode)
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
