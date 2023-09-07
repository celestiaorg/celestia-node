package share

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/rsmt2d"
)

// EmptyRoot returns Root of the empty block EDS.
func EmptyRoot() *Root {
	initEmpty()
	return emptyBlockRoot
}

// EmptyExtendedDataSquare returns the EDS of the empty block data square.
func EmptyExtendedDataSquare() *rsmt2d.ExtendedDataSquare {
	initEmpty()
	return emptyBlockEDS
}

// EmptyBlockShares returns the shares of the empty block.
func EmptyBlockShares() []Share {
	initEmpty()
	return emptyBlockShares
}

var (
	emptyMu          sync.Mutex
	emptyBlockRoot   *Root
	emptyBlockEDS    *rsmt2d.ExtendedDataSquare
	emptyBlockShares []Share
)

// initEmpty enables lazy initialization for constant empty block data.
func initEmpty() {
	emptyMu.Lock()
	defer emptyMu.Unlock()
	if emptyBlockRoot != nil {
		return
	}

	// compute empty block EDS and DAH for it
	result := shares.TailPaddingShares(appconsts.MinShareCount)
	emptyBlockShares = shares.ToBytes(result)

	eds, err := da.ExtendShares(emptyBlockShares)
	if err != nil {
		panic(fmt.Errorf("failed to create empty EDS: %w", err))
	}
	emptyBlockEDS = eds

	dah, err := da.NewDataAvailabilityHeader(eds)
	if err != nil {
		panic(fmt.Errorf("failed to create empty DAH: %w", err))
	}
	minDAH := da.MinDataAvailabilityHeader()
	if !bytes.Equal(minDAH.Hash(), dah.Hash()) {
		panic(fmt.Sprintf("mismatch in calculated minimum DAH and minimum DAH from celestia-app, "+
			"expected %s, got %s", minDAH.String(), dah.String()))
	}
	emptyBlockRoot = &dah

	// precompute Hash, so it's cached internally to avoid potential races
	emptyBlockRoot.Hash()
}
