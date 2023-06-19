package ipld

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
)

var ErrNamespaceOutsideRange = errors.New("share/ipld: " +
	"target namespace is outside of namespace range for the given root")

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
	leaves []ipld.Node
	proofs *proofCollector

	bounds    fetchedBounds
	maxShares int
	namespace share.Namespace

	isAbsentNamespace atomic.Bool
	absenceProofLeaf  ipld.Node
}

func NewNamespaceData(maxShares int, namespace share.Namespace, options ...Option) *NamespaceData {
	data := &NamespaceData{
		// we don't know where in the tree the leaves in the namespace are,
		// so we keep track of the bounds to return the correct slice
		// maxShares acts as a sentinel to know if we find any leaves
		bounds:    fetchedBounds{int64(maxShares), 0},
		maxShares: maxShares,
		namespace: namespace,
	}

	for _, opt := range options {
		opt(data)
	}
	return data
}

func (n *NamespaceData) validate(rootCid cid.Cid) error {
	if err := n.namespace.Validate(); err != nil {
		return err
	}

	if n.leaves == nil && n.proofs == nil {
		return errors.New("share/ipld: empty NamespaceData, nothing specified to retrieve")
	}

	root := NamespacedSha256FromCID(rootCid)
	if n.namespace.IsOutsideRange(root, root) {
		return ErrNamespaceOutsideRange
	}
	return nil
}

func (n *NamespaceData) addLeaf(pos int, nd ipld.Node) {
	// bounds will be needed in `Proof` method
	n.bounds.update(int64(pos))

	if n.isAbsentNamespace.Load() {
		if n.absenceProofLeaf != nil {
			log.Fatal("there should be only one absence leaf")
		}
		n.absenceProofLeaf = nd
		return
	}

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

// Leaves returns retrieved leaves within the bounds in case `WithLeaves` option was passed,
// otherwise nil will be returned.
func (n *NamespaceData) Leaves() []ipld.Node {
	if n.leaves == nil || n.noLeaves() || n.isAbsentNamespace.Load() {
		return nil
	}
	return n.leaves[n.bounds.lowest : n.bounds.highest+1]
}

// Proof returns proofs within the bounds in case if `WithProofs` option was passed,
// otherwise nil will be returned.
func (n *NamespaceData) Proof() *nmt.Proof {
	if n.proofs == nil {
		return nil
	}

	// return an empty Proof if leaves are not available
	if n.noLeaves() {
		return &nmt.Proof{}
	}

	nodes := make([][]byte, len(n.proofs.Nodes()))
	for i, node := range n.proofs.Nodes() {
		nodes[i] = NamespacedSha256FromCID(node)
	}

	if n.isAbsentNamespace.Load() {
		proof := nmt.NewAbsenceProof(
			int(n.bounds.lowest),
			int(n.bounds.highest)+1,
			nodes,
			NamespacedSha256FromCID(n.absenceProofLeaf.Cid()),
			NMTIgnoreMaxNamespace,
		)
		return &proof
	}
	proof := nmt.NewInclusionProof(
		int(n.bounds.lowest),
		int(n.bounds.highest)+1,
		nodes,
		NMTIgnoreMaxNamespace,
	)
	return &proof
}

// CollectLeavesByNamespace collects leaves and corresponding proof that could be used to verify
// leaves inclusion. It returns as many leaves from the given root with the given Namespace as
// it can retrieve. If no shares are found, it returns error as nil. A
// non-nil error means that only partial data is returned, because at least one share retrieval
// failed. The following implementation is based on `GetShares`.
func (n *NamespaceData) CollectLeavesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
) error {
	if err := n.validate(root); err != nil {
		return err
	}

	ctx, span := tracer.Start(ctx, "get-leaves-by-namespace")
	defer span.End()

	span.SetAttributes(
		attribute.String("namespace", n.namespace.String()),
		attribute.String("root", root.String()),
	)

	// buffer the jobs to avoid blocking, we only need as many
	// queued as the number of shares in the second-to-last layer
	jobs := make(chan job, (n.maxShares+1)/2)
	jobs <- job{cid: root, ctx: ctx}

	var wg chanGroup
	wg.jobs = jobs
	wg.add(1)

	var (
		singleErr    sync.Once
		retrievalErr error
	)

	for {
		var j job
		var ok bool
		select {
		case j, ok = <-jobs:
		case <-ctx.Done():
			return ctx.Err()
		}

		if !ok {
			return retrievalErr
		}
		pool.Submit(func() {
			ctx, span := tracer.Start(j.ctx, "process-job")
			defer span.End()
			defer wg.done()

			span.SetAttributes(
				attribute.String("cid", j.cid.String()),
				attribute.Int("pos", j.sharePos),
			)

			// if an error is likely to be returned or not depends on
			// the underlying impl of the blockservice, currently it is not a realistic probability
			nd, err := GetNode(ctx, bGetter, j.cid)
			if err != nil {
				singleErr.Do(func() {
					retrievalErr = err
				})
				log.Errorw("could not retrieve IPLD node",
					"namespace", n.namespace.String(),
					"pos", j.sharePos,
					"err", err,
				)
				span.SetStatus(codes.Error, err.Error())
				// we still need to update the bounds
				n.addLeaf(j.sharePos, nil)
				return
			}

			links := nd.Links()
			if len(links) == 0 {
				// successfully fetched a leaf belonging to the namespace
				span.SetStatus(codes.Ok, "")
				// we found a leaf, so we update the bounds
				n.addLeaf(j.sharePos, nd)
				return
			}

			// this node has links in the namespace, so keep walking
			newJobs := n.traverseLinks(j, links)
			for _, j := range newJobs {
				wg.add(1)
				select {
				case jobs <- j:
				case <-ctx.Done():
					return
				}
			}
		})
	}
}

func (n *NamespaceData) traverseLinks(j job, links []*ipld.Link) []job {
	if j.isAbsent {
		return n.collectAbsenceProofs(j, links)
	}
	return n.collectNDWithProofs(j, links)
}

func (n *NamespaceData) collectAbsenceProofs(j job, links []*ipld.Link) []job {
	leftLink := links[0].Cid
	rightLink := links[1].Cid
	// traverse to the left node, while collecting right node as proof
	n.addProof(right, rightLink, j.depth)
	return []job{j.next(left, leftLink, j.isAbsent)}
}

func (n *NamespaceData) collectNDWithProofs(j job, links []*ipld.Link) []job {
	leftCid := links[0].Cid
	rightCid := links[1].Cid
	leftLink := NamespacedSha256FromCID(leftCid)
	rightLink := NamespacedSha256FromCID(rightCid)

	var nextJobs []job
	// check if target namespace is outside of boundaries of both links
	if n.namespace.IsOutsideRange(leftLink, rightLink) {
		log.Fatalf("target namespace outside of boundaries of links at depth: %v", j.depth)
	}

	if !n.namespace.IsAboveMax(leftLink) {
		// namespace is within the range of left link
		nextJobs = append(nextJobs, j.next(left, leftCid, false))
	} else {
		// proof is on the left side, if the namespace is on the right side of the range of left link
		n.addProof(left, leftCid, j.depth)
		if n.namespace.IsBelowMin(rightLink) {
			// namespace is not included in either links, convert to absence collector
			n.isAbsentNamespace.Store(true)
			nextJobs = append(nextJobs, j.next(right, rightCid, true))
			return nextJobs
		}
	}

	if !n.namespace.IsBelowMin(rightLink) {
		// namespace is within the range of right link
		nextJobs = append(nextJobs, j.next(right, rightCid, false))
	} else {
		// proof is on the right side, if the namespace is on the left side of the range of right link
		n.addProof(right, rightCid, j.depth)
	}
	return nextJobs
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
