package cache

import (
	"testing"

	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
)

// randLightLocalServiceWithSquare is the same as randLightServiceWithSquare, except
// the share.Availability is wrapped with cache availability.
func randLightLocalServiceWithSquare(t *testing.T, n int) (*availability.Service, *share.Root) {
	bServ := mdutils.Bserv()
	store := dssync.MutexWrap(ds.NewMapDatastore())
	ca := NewShareAvailability(
		light.TestLightAvailability(bServ),
		store,
	)
	return availability.NewService(bServ, ca), availability_test.RandFillBS(t, n, bServ)
}

// randFullLocalServiceWithSquare is the same as randFullServiceWithSquare, except
// the share.Availability is wrapped with cache availability.
func randFullLocalServiceWithSquare(t *testing.T, n int) (*availability.Service, *share.Root) {
	bServ := mdutils.Bserv()
	store := dssync.MutexWrap(ds.NewMapDatastore())
	ca := NewShareAvailability(
		full.TestFullAvailability(bServ),
		store,
	)
	return availability.NewService(bServ, ca), availability_test.RandFillBS(t, n, bServ)
}
