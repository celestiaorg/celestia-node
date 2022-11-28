package ipld

import (
	"math"
)

// Proof contains information required for Leaves inclusion validation.
type Proof struct {
	Nodes      [][]byte
	Start, End int
}

// proofCollector collects proof nodes with corresponding indexes and provides Nodes array that
// could be used to construct nmt.Proof for shares inclusion validation.
type proofCollector struct {
	leftProofs, rightProofs [][]byte
}

func newProofCollector(maxShares int) *proofCollector {
	// maximum possible amount of required proofs from each side is equal to tree height.
	height := int(math.Log2(float64(maxShares)))
	return &proofCollector{
		leftProofs:  make([][]byte, 0, height),
		rightProofs: make([][]byte, 0, height),
	}
}

func (c *proofCollector) addLeft(node []byte) {
	c.leftProofs = append(c.leftProofs, node)
}

func (c *proofCollector) addRight(node []byte) {
	c.rightProofs = append(c.rightProofs, node)
}

// Nodes returns nodes collected by proofCollector in the order that nmt.Proof validator will use
// to traverse the tree.
func (c *proofCollector) Nodes() [][]byte {
	nodes := make([][]byte, 0, len(c.leftProofs)+len(c.leftProofs))
	// left side will be traversed in bottom-up order
	nodes = append(nodes, c.leftProofs...)

	// right side of the tree will be traversed from top to bottom,
	// so sort in reversed order
	for i := len(c.rightProofs) - 1; i >= 0; i-- {
		nodes = append(nodes, c.rightProofs[i])
	}
	return nodes
}
