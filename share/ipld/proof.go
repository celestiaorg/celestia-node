package ipld

import (
	"math"
	"sort"
	"sync"
)

// Proof contains information required for Leaves inclusion validation.
type Proof struct {
	Nodes      [][]byte
	Start, End int
}

// proofCollector collects proof nodes with corresponding indexes and provides Nodes array that
// could be used to construct nmt.Proof for shares inclusion validation.
type proofCollector struct {
	leftProofs, rightProofs []nodeWithIdx
	leftLk, rightLk         sync.Mutex
}

type nodeWithIdx struct {
	idx  int
	node []byte
}

func newProofCollector(maxShares int) *proofCollector {
	// maximum possible amount of required proofs from each side is equal to tree height.
	height := int(math.Log2(float64(maxShares)))
	return &proofCollector{
		leftProofs:  make([]nodeWithIdx, 0, height),
		rightProofs: make([]nodeWithIdx, 0, height),
	}
}

func (c *proofCollector) addLeft(idx int, node []byte) {
	c.leftLk.Lock()
	defer c.leftLk.Unlock()
	c.leftProofs = append(c.leftProofs, nodeWithIdx{
		idx:  idx,
		node: node,
	})
}

func (c *proofCollector) addRight(idx int, node []byte) {
	c.rightLk.Lock()
	defer c.rightLk.Unlock()
	c.rightProofs = append(c.rightProofs, nodeWithIdx{
		idx:  idx,
		node: node,
	})
}

// Nodes returns nodes collected by proofCollector in the order that nmt.Proof validator will use
// to traverse the tree.
func (c *proofCollector) Nodes() [][]byte {
	// left side will be traversed in bottom-up order
	sort.Slice(c.leftProofs, func(i, j int) bool {
		return c.leftProofs[i].idx < c.leftProofs[j].idx
	})

	// right side of the tree will be traversed from top to bottom,
	// so sort in reversed order
	sort.Slice(c.rightProofs, func(i, j int) bool {
		return c.rightProofs[i].idx > c.rightProofs[j].idx
	})

	nodes := make([][]byte, 0, len(c.leftProofs)+len(c.leftProofs))
	for _, n := range c.leftProofs {
		nodes = append(nodes, n.node)
	}
	for _, n := range c.rightProofs {
		nodes = append(nodes, n.node)
	}
	return nodes
}
