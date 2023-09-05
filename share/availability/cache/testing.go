package cache

import (
	"testing"

	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

// LightAvailabilityWithLocalRandSquare wraps light.GetterWithRandSquare with cache availability
func LightAvailabilityWithLocalRandSquare(t *testing.T, n int) (share.Availability, *share.Root) {
	bServ := ipld.NewMemBlockservice()
	store := dssync.MutexWrap(ds.NewMapDatastore())
	getter := getters.NewIPLDGetter(bServ)
	avail := NewShareAvailability(
		light.TestAvailability(getter),
		store,
	)
	return avail, availability_test.RandFillBS(t, n, bServ)
}

// FullAvailabilityWithLocalRandSquare wraps full.GetterWithRandSquare with cache availability
func FullAvailabilityWithLocalRandSquare(t *testing.T, n int) (share.Availability, *share.Root) {
	bServ := ipld.NewMemBlockservice()
	store := dssync.MutexWrap(ds.NewMapDatastore())
	getter := getters.NewIPLDGetter(bServ)
	avail := NewShareAvailability(
		full.TestAvailability(getter),
		store,
	)
	return avail, availability_test.RandFillBS(t, n, bServ)
}
