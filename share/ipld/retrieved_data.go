package ipld

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

var errNoLeavesAvailable = errors.New("no leaves available under the given root")

type Option func(*RetrievedData)

// WithLeaves option specifies that leaves should be collected during retrieval.
func WithLeaves() Option {
	return func(data *RetrievedData) {
		// we over-allocate space for leaves since we do not know how many we will find
		// on the level above, the length of the Row is passed in as maxShares
		data.leaves = make([]ipld.Node, data.maxShares)
	}
}

// WithProofs option specifies that proofs should be collected during retrieval.
func WithProofs() Option {
	return func(data *RetrievedData) {
		data.proofContainer = newProofCollector(data.maxShares)
	}
}

type RetrievedData struct {
	leaves         []ipld.Node
	proofContainer *proofCollector
	bounds         fetchedBounds
	maxShares      int
	err            error
}

func NewRetrievedData(maxShares int, options ...Option) *RetrievedData {
	rData := &RetrievedData{
		// we don't know where in the tree the leaves in the namespace are,
		// so we keep track of the bounds to return the correct slice
		// maxShares acts as a sentinel to know if we find any leaves
		bounds:    fetchedBounds{int64(maxShares), 0},
		maxShares: maxShares,
	}

	for _, opt := range options {
		opt(rData)
	}
	return rData
}

func (r *RetrievedData) validateBasic() error {
	if r.leaves == nil && r.proofContainer == nil {
		return errors.New("share/ipld: Invalid configuration: not specified what to retrieve")
	}
	return nil
}

func (r *RetrievedData) getMaxShares() int {
	return r.maxShares
}

func (r *RetrievedData) addLeaf(pos int, nd ipld.Node) {
	// bounds will be needed in `collectProofs`
	r.bounds.update(int64(pos))

	if r.leaves == nil {
		return
	}

	if nd != nil {
		r.leaves[pos] = nd
	}
}

// leavesAvailable ensures if there leaves under the given root in the given namespace.
func (r *RetrievedData) leavesAvailable() bool {
	if r.bounds.lowest == int64(r.maxShares) {
		// reset slice if no leaves available under the given root
		r.leaves = nil
		r.err = errNoLeavesAvailable
		return false
	}
	return true
}

type direction int

const (
	left direction = iota + 1
	right
)

func (r *RetrievedData) addProof(d direction, cid cid.Cid, depth int) {
	if r.proofContainer == nil {
		return
	}

	switch d {
	case left:
		r.proofContainer.addLeft(cid, depth)
	case right:
		r.proofContainer.addRight(cid, depth)
	default:
		panic(fmt.Sprintf("share/ipld: invalid direction: %d", d))
	}
}

// CollectLeaves returns retrieved leaves within the bounds in case if `WithLeaves` option was passed,
// otherwise nil will be returned.
func (r *RetrievedData) CollectLeaves() []ipld.Node {
	if r.leaves == nil {
		return nil
	}
	return r.leaves[r.bounds.lowest : r.bounds.highest+1]
}

// CollectProofs returns proofs within the bounds in case if `WithProofs` option was passed,
// otherwise nil will be returned.
func (r *RetrievedData) CollectProofs() *Proof {
	if r.proofContainer == nil {
		return nil
	}

	if r.err != nil && errors.Is(r.err, errNoLeavesAvailable) {
		return &Proof{}
	}

	return &Proof{
		Start: int(r.bounds.lowest),
		End:   int(r.bounds.highest) + 1,
		Nodes: r.proofContainer.Nodes(),
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
