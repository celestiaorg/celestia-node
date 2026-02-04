package share

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/celestiaorg/celestia-app/v7/pkg/da"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"
)

// EmptyEDSDataHash returns DataHash of the empty block EDS.
func EmptyEDSDataHash() DataHash {
	initEmpty()
	return emptyBlockDataHash
}

// EmptyEDSRoots returns AxisRoots of the empty block EDS.
func EmptyEDSRoots() *AxisRoots {
	initEmpty()
	return emptyBlockRoots
}

// EmptyEDS returns the EDS of the empty block data square.
func EmptyEDS() *rsmt2d.ExtendedDataSquare {
	initEmpty()
	return emptyBlockEDS
}

var (
	emptyOnce          sync.Once
	emptyBlockDataHash DataHash
	emptyBlockRoots    *AxisRoots
	emptyBlockEDS      *rsmt2d.ExtendedDataSquare
)

// initEmpty enables lazy initialization for constant empty block data.
func initEmpty() {
	emptyOnce.Do(computeEmpty)
}

func computeEmpty() {
	// compute empty block EDS and DAH for it
	result := libshare.TailPaddingShares(libshare.MinShareCount)
	rawEmptyBlockShares := libshare.ToBytes(result)

	eds, err := da.ExtendShares(rawEmptyBlockShares)
	if err != nil {
		panic(fmt.Errorf("failed to create empty EDS: %w", err))
	}
	emptyBlockEDS = eds

	emptyBlockRoots, err = NewAxisRoots(eds)
	if err != nil {
		panic(fmt.Errorf("failed to create empty DAH: %w", err))
	}
	minDAH := da.MinDataAvailabilityHeader()
	if !bytes.Equal(minDAH.Hash(), emptyBlockRoots.Hash()) {
		panic(fmt.Sprintf("mismatch in calculated minimum DAH and minimum DAH from celestia-app, "+
			"expected %s, got %s", minDAH.String(), emptyBlockRoots.String()))
	}

	// precompute Hash, so it's cached internally to avoid potential races
	emptyBlockDataHash = emptyBlockRoots.Hash()
}
