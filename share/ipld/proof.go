package ipld

import (
	"math"
	"sort"
	"sync"

	"github.com/ipfs/go-cid"
)

// Proof contains information required for Leaves inclusion validation.
type Proof struct {
	Nodes      []cid.Cid
	Start, End int
}

// proofCollector collects proof nodes' CIDs for the construction of a shares inclusion validation
// nmt.Proof.
type proofCollector struct {
	leftMu, rightMu sync.Mutex
	left, right     []node
}

type node struct {
	cid   cid.Cid
	depth int
}

func newProofCollector(maxShares int) *proofCollector {
	// maximum possible amount of required proofs from each side is equal to tree height.
	height := int(math.Log2(float64(maxShares)))
	return &proofCollector{
		left:  make([]node, 0, height),
		right: make([]node, 0, height),
	}
}

func (c *proofCollector) addLeft(cid cid.Cid, depth int) {
	c.leftMu.Lock()
	defer c.leftMu.Unlock()
	c.left = append(c.left, node{
		cid:   cid,
		depth: depth,
	})
}

func (c *proofCollector) addRight(cid cid.Cid, depth int) {
	c.rightMu.Lock()
	defer c.rightMu.Unlock()
	c.right = append(c.right, node{
		cid:   cid,
		depth: depth,
	})
}

// Nodes returns nodes collected by proofCollector in the order that nmt.Proof validator will use
// to traverse the tree.
func (c *proofCollector) Nodes() []cid.Cid {
	cids := make([]cid.Cid, 0, len(c.left)+len(c.right))
	// left side will be traversed in bottom-up order
	sort.Slice(c.left, func(i, j int) bool {
		return c.left[i].depth < c.left[j].depth
	})
	for _, node := range c.left {
		cids = append(cids, node.cid)
	}

	// right side of the tree will be traversed from top to bottom,
	// so sort in reversed order
	sort.Slice(c.right, func(i, j int) bool {
		return c.right[i].depth > c.right[j].depth
	})
	for _, node := range c.right {
		cids = append(cids, node.cid)
	}
	return cids
}
