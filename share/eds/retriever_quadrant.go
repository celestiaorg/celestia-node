package eds

import (
	"math/rand"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/ipld"
)

const (
	// there are always 4 quadrants
	numQuadrants = 4
	// blockTime equals to the time with which new blocks are produced in the network.
	// TODO(@Wondertan): Here we assume that the block time is a minute, but
	//  block time is a network wide variable/param that has to be taken from
	//  a proper place
	blockTime = time.Minute
)

// RetrieveQuadrantTimeout defines how much time Retriever waits before
// starting to retrieve another quadrant.
//
// NOTE:
// - The whole data square must be retrieved in less than block time.
// - We have 4 quadrants from two sources(rows, cols) which equals to 8 in total.
var RetrieveQuadrantTimeout = blockTime / numQuadrants * 2

type quadrant struct {
	// slice of roots to get shares from
	roots []cid.Cid
	// Example coordinates(x;y) of each quadrant when fetching from column roots
	// ------  -------
	// |  Q0 | | Q1  |
	// |(0;0)| |(1;0)|
	// ------  -------
	// |  Q2 | | Q3  |
	// |(0;1)| |(1;1)|
	// ------  -------
	x, y int
	// source defines the axis(Row or Col) to fetch the quadrant from
	source rsmt2d.Axis
}

// newQuadrants constructs a slice of quadrants from DAHeader.
// There are always 4 quadrants per each source (row and col), so 8 in total.
// The ordering of quadrants is random.
func newQuadrants(dah *da.DataAvailabilityHeader) []*quadrant {
	// combine all the roots into one slice, so they can be easily accessible by index
	daRoots := [][][]byte{
		dah.RowsRoots,
		dah.ColumnRoots,
	}
	// create a quadrant slice for each source(row;col)
	sources := [][]*quadrant{
		make([]*quadrant, numQuadrants),
		make([]*quadrant, numQuadrants),
	}
	for source, quadrants := range sources {
		size, qsize := len(daRoots[source]), len(daRoots[source])/2
		roots := make([]cid.Cid, size)
		for i, root := range daRoots[source] {
			roots[i] = ipld.MustCidFromNamespacedSha256(root)
		}

		for i := range quadrants {
			// convert quadrant 1D into into 2D coordinates
			x, y := i%2, i/2
			quadrants[i] = &quadrant{
				roots:  roots[qsize*y : qsize*(y+1)],
				x:      x,
				y:      y,
				source: rsmt2d.Axis(source),
			}
		}
	}
	quadrants := make([]*quadrant, 0, numQuadrants*2)
	for _, qs := range sources {
		quadrants = append(quadrants, qs...)
	}
	// shuffle quadrants to be fetched in random order
	rand.Shuffle(len(quadrants), func(i, j int) { quadrants[i], quadrants[j] = quadrants[j], quadrants[i] })
	return quadrants
}

// pos calculates position of a share in a data square.
func (q *quadrant) pos(rootIdx, cellIdx int) (int, int) {
	cellIdx += len(q.roots) * q.x
	rootIdx += len(q.roots) * q.y
	switch q.source {
	case rsmt2d.Row:
		return rootIdx, cellIdx
	case rsmt2d.Col:
		return cellIdx, rootIdx
	default:
		panic("unknown axis")
	}
}
