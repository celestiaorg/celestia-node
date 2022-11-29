package ipld

import (
	"math"

	"github.com/ipfs/go-cid"
)

// Proof contains information required for Leaves inclusion validation.
type Proof struct {
	Nodes      []cid.Cid
	Start, End int
}

// proofCollector collects proof nodes' CIDs for the construction of a shares inclusion validation nmt.Proof.
type proofCollector struct {
	left, right []cid.Cid
}

func newProofCollector(maxShares int) *proofCollector {
	// maximum possible amount of required proofs from each side is equal to tree height.
	height := int(math.Log2(float64(maxShares)))
	return &proofCollector{
		left:  make([]cid.Cid, 0, height),
		right: make([]cid.Cid, 0, height),
	}
}

func (c *proofCollector) addLeft(node cid.Cid) {
	c.left = append(c.left, node)
}

func (c *proofCollector) addRight(node cid.Cid) {
	c.right = append(c.right, node)
}

// Nodes returns nodes collected by proofCollector in the order that nmt.Proof validator will use
// to traverse the tree.
func (c *proofCollector) Nodes() []cid.Cid {
	nodes := make([]cid.Cid, 0, len(c.left)+len(c.left))
	// left side will be traversed in bottom-up order
	nodes = append(nodes, c.left...)

	// right side of the tree will be traversed from top to bottom,
	// so sort in reversed order
	for i := len(c.right) - 1; i >= 0; i-- {
		nodes = append(nodes, c.right[i])
	}
	return nodes
}
