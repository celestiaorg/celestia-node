package ipld

import (
	"math"
	"math/rand"

	"github.com/ipfs/go-cid"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
)

// there are always 4 quadrants
const numQuadrants = 4

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
	// source defines the axis for quadrant
	// it can be either 1 or 0 similar to x and y
	// where 0 is Row source and 1 is Col respectively
	source int
}

// newQuadrants constructs a slice of quadrants from DAHeader.
// Constantly, there are 4 quadrants for each source: row and col (8 in total).
// The ordering of quadrants is random.
func newQuadrants(dah *da.DataAvailabilityHeader) [][]*quadrant {
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
			roots[i] = plugin.MustCidFromNamespacedSha256(root)
		}

		for i := range quadrants {
			// convert quadrant index into coordinates
			x, y := i%2, i>>1
			if source == 1 { // swap coordinates for column
				x, y = y, x
			}

			quadrants[i] = &quadrant{
				roots:  roots[qsize*y : qsize*(y+1)],
				x:      x,
				y:      y,
				source: source,
			}
		}
		// shuffle quadrants to be fetched in random order
		rand.Shuffle(len(quadrants), func(i, j int) { quadrants[i], quadrants[j] = quadrants[j], quadrants[i] })
	}
	// randomize sources, s.t. downloading on the network happens from both rows and cols
	rand.Shuffle(len(sources), func(i, j int) { sources[i], sources[j] = sources[j], sources[i] })
	return sources
}

// index calculates index for a share in a data square slice flattened by rows.
//
// NOTE: The complexity of the formula below comes from:
//  * Goal to avoid share copying
//  * Goal to make formula generic for both rows and cols
//  	* While data square is flattened by rows only
// TODO(@Wondertan): This can be simplified by making rsmt2d working over 3D byte slice(not flattened)
func (q *quadrant) index(rootIdx, cellIdx int) int {
	size := len(q.roots)
	// half square offsets, e.g. share is from Q3,
	// so we add to index Q1+Q2
	halfSquareOffsetCol := pow(size*2, q.source)
	halfSquareOffsetRow := pow(size*2, q.source^1)
	// offsets for the axis, e.g. share is from Q4.
	// so we add to index Q3
	offsetX := q.x * halfSquareOffsetCol * size
	offsetY := q.y * halfSquareOffsetRow * size

	rootIdx *= halfSquareOffsetRow
	cellIdx *= halfSquareOffsetCol
	return rootIdx + cellIdx + offsetX + offsetY
}

func pow(x, y int) int {
	return int(math.Pow(float64(x), float64(y)))
}
