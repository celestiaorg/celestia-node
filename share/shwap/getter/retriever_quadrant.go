package shwap_getter

import (
	"time"
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

// RetrieveQuadrantTimeout defines how much time edsRetriver waits before
// starting to retrieve another quadrant.
//
// NOTE:
// - The whole data square must be retrieved in less than block time.
// - We have 4 quadrants from two sources(rows, cols) which equals to 8 in total.
var RetrieveQuadrantTimeout = blockTime / numQuadrants * 2

type quadrant struct {
	// Example coordinates(x;y) of each quadrant
	// ------  -------
	// |  Q0 | | Q1  |
	// |(0;0)| |(1;0)|
	// ------  -------
	// |  Q2 | | Q3  |
	// |(0;1)| |(1;1)|
	// ------  -------
	x, y int
}

// newQuadrants constructs a slice of quadrants. There are always 4 quadrants.
func newQuadrants() []quadrant {
	quadrants := make([]quadrant, 0, numQuadrants)
	for i := 0; i < numQuadrants; i++ {
		quadrants = append(quadrants, quadrant{x: i % 2, y: i / 2})
	}
	return quadrants
}
