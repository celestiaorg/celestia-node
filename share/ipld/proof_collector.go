package ipld

import (
	"math"

	"github.com/ipfs/go-cid"
)

// proofCollector collects proof nodes' CIDs for the construction of a shares inclusion validation
// nmt.Proof.
type proofCollector struct {
	left, right []cid.Cid
}

func newProofCollector(maxShares int) *proofCollector {
	// maximum possible amount of required proofs from each side is equal to tree height.
	height := int(math.Log2(float64(maxShares))) + 1
	return &proofCollector{
		left:  make([]cid.Cid, height),
		right: make([]cid.Cid, height),
	}
}

func (c *proofCollector) addLeft(cid cid.Cid, depth int) {
	c.left[depth] = cid
}

func (c *proofCollector) addRight(cid cid.Cid, depth int) {
	c.right[depth] = cid
}

// Nodes returns nodes collected by proofCollector in the order that nmt.Proof validator will use
// to traverse the tree.
func (c *proofCollector) Nodes() []cid.Cid {
	cids := make([]cid.Cid, 0, len(c.left)+len(c.right))
	// left side will be traversed in bottom-up order
	for _, cid := range c.left {
		if cid.Defined() {
			cids = append(cids, cid)
		}
	}

	// right side of the tree will be traversed from top to bottom,
	// so sort in reversed order
	for i := len(c.right) - 1; i >= 0; i-- {
		cid := c.right[i]
		if cid.Defined() {
			cids = append(cids, cid)
		}
	}
	return cids
}
