package ipld

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/nmt/namespace"
)

// Option is the functional option that is applied to the NamespaceData instance
// to configure data that needs to be stored.
type Option func(*NamespaceData)

// WithLeaves option specifies that leaves should be collected during retrieval.
func WithLeaves() Option {
	return func(data *NamespaceData) {
		// we over-allocate space for leaves since we do not know how many we will find
		// on the level above, the length of the Row is passed in as maxShares
		data.leaves = make([]ipld.Node, data.maxShares)
	}
}

// WithProofs option specifies that proofs should be collected during retrieval.
func WithProofs() Option {
	return func(data *NamespaceData) {
		data.proofs = newProofCollector(data.maxShares)
	}
}

// NamespaceData stores all leaves under the given namespace with their corresponding proofs.
type NamespaceData struct {
	leaves    []ipld.Node
	proofs    *proofCollector
	bounds    fetchedBounds
	maxShares int
	nID       namespace.ID
}

func NewNamespaceData(maxShares int, nID namespace.ID, options ...Option) *NamespaceData {
	rData := &NamespaceData{
		// we don't know where in the tree the leaves in the namespace are,
		// so we keep track of the bounds to return the correct slice
		// maxShares acts as a sentinel to know if we find any leaves
		bounds:    fetchedBounds{int64(maxShares), 0},
		maxShares: maxShares,
		nID:       nID,
	}

	for _, opt := range options {
		opt(rData)
	}
	return rData
}

func (n *NamespaceData) validate() error {
	if len(n.nID) != NamespaceSize {
		return fmt.Errorf("expected namespace ID of size %d, got %d", NamespaceSize, len(n.nID))
	}

	if n.leaves == nil && n.proofs == nil {
		return errors.New("share/ipld: empty NamespaceData, nothing specified to retrieve")
	}
	return nil
}

func (n *NamespaceData) addLeaf(pos int, nd ipld.Node) {
	// bounds will be needed in `collectProofs`
	n.bounds.update(int64(pos))

	if n.leaves == nil {
		return
	}

	if nd != nil {
		n.leaves[pos] = nd
	}
}

// noLeaves checks that there are no leaves under the given root in the given namespace.
func (n *NamespaceData) noLeaves() bool {
	return n.bounds.lowest == int64(n.maxShares)
}

type direction int

const (
	left direction = iota + 1
	right
)

func (n *NamespaceData) addProof(d direction, cid cid.Cid, depth int) {
	if n.proofs == nil {
		return
	}

	switch d {
	case left:
		n.proofs.addLeft(cid, depth)
	case right:
		n.proofs.addRight(cid, depth)
	default:
		panic(fmt.Sprintf("share/ipld: invalid direction: %d", d))
	}
}

// CollectLeaves returns retrieved leaves within the bounds in case `WithLeaves` option was passed,
// otherwise nil will be returned.
func (n *NamespaceData) CollectLeaves() []ipld.Node {
	if n.leaves == nil || n.noLeaves() {
		return nil
	}
	return n.leaves[n.bounds.lowest : n.bounds.highest+1]
}

// CollectProofs returns proofs within the bounds in case if `WithProofs` option was passed,
// otherwise nil will be returned.
func (n *NamespaceData) CollectProofs() *Proof {
	if n.proofs == nil {
		return nil
	}

	// return an empty Proof if leaves are not available
	if n.noLeaves() {
		return &Proof{}
	}

	return &Proof{
		Start: int(n.bounds.lowest),
		End:   int(n.bounds.highest) + 1,
		Nodes: n.proofs.Nodes(),
	}
}

type fetchedBounds struct {
	lowest  int64
	highest int64
}

// update checks if the passed index is outside the current bounds,
// and updates the bounds atomically if it extends them.
func (b *fetchedBounds) update(index int64) {
	lowest := atomic.LoadInt64(&b.lowest)
	// try to write index to the lower bound if appropriate, and retry until the atomic op is successful
	// CAS ensures that we don't overwrite if the bound has been updated in another goroutine after the
	// comparison here
	for index < lowest && !atomic.CompareAndSwapInt64(&b.lowest, lowest, index) {
		lowest = atomic.LoadInt64(&b.lowest)
	}
	// we always run both checks because element can be both the lower and higher bound
	// for example, if there is only one share in the namespace
	highest := atomic.LoadInt64(&b.highest)
	for index > highest && !atomic.CompareAndSwapInt64(&b.highest, highest, index) {
		highest = atomic.LoadInt64(&b.highest)
	}
}
