package ipld

import (
	"fmt"
	"hash"

	"github.com/minio/sha256-simd"
	mhcore "github.com/multiformats/go-multihash/core"

	"github.com/celestiaorg/nmt"
)

func init() {
	// Register custom hasher in the multihash register.
	// Required for the Bitswap to hash and verify inbound data correctly
	mhcore.Register(sha256Namespace8Flagged, func() hash.Hash {
		return defaultHasher()
	})
}

// namespaceHasher implements hash.Hash over NMT Hasher.
// TODO: Move to NMT repo!
//
//	As NMT already defines Hasher that implements hash.Hash by embedding,
//	it should define full and *correct* implementation of hash.Hash,
//	which is the namespaceHasher. This hash.Hash, is not IPLD specific and
//	can be used elsewhere.
type namespaceHasher struct {
	hasher *nmt.Hasher

	tp   byte   // keeps type of NMT node to be hashed
	data []byte // written data of the NMT node
}

// defaultHasher constructs the namespaceHasher with default configuration
func defaultHasher() *namespaceHasher {
	nh := &namespaceHasher{hasher: nmt.NewNmtHasher(sha256.New(), NamespaceSize, true)}
	nh.Reset()
	return nh
}

// Size returns the number of bytes Sum will return.
func (n *namespaceHasher) Size() int {
	return n.hasher.Size() + int(n.hasher.NamespaceLen)*2
}

// Write writes the namespaced data to be hashed.
//
// Requires data of fixed size to match leaf or inner NMT nodes.
// Only one write is allowed.
func (n *namespaceHasher) Write(data []byte) (int, error) {
	if n.data != nil {
		panic("namespaceHasher: only single Write is allowed")
	}

	ln := len(data)
	switch ln {
	default:
		log.Warnf("unexpected data size: %d", ln)
		return 0, fmt.Errorf("wrong sized data written to the hasher, len: %v", ln)
	case innerNodeSize:
		n.tp = nmt.NodePrefix
	case leafNodeSize:
		n.tp = nmt.LeafPrefix
	}

	n.data = data
	return ln, nil
}

// Sum computes the hash.
// Does not append the given suffix and violating the interface.
func (n *namespaceHasher) Sum([]byte) []byte {
	switch n.tp {
	case nmt.LeafPrefix:
		return n.hasher.HashLeaf(n.data)
	case nmt.NodePrefix:
		flagLen := int(n.hasher.NamespaceLen * 2)
		sha256Len := n.hasher.Size()
		return n.hasher.HashNode(n.data[:flagLen+sha256Len], n.data[flagLen+sha256Len:])
	default:
		panic("namespaceHasher: node type wasn't set")
	}
}

// Reset resets the Hash to its initial state.
func (n *namespaceHasher) Reset() {
	n.tp, n.data = 255, nil // reset with an invalid node type, as zero value is a valid Leaf
	n.hasher.Reset()
}

// BlockSize returns the hash's underlying block size.
func (n *namespaceHasher) BlockSize() int {
	return n.hasher.BlockSize()
}
