package cache

import (
	"testing"

	"github.com/celestiaorg/celestia-node/share/service"

	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
)

// RandLightLocalServiceWithSquare is the same as randLightServiceWithSquare, except
// the share.Availability is wrapped with cache availability.
func RandLightLocalServiceWithSquare(t *testing.T, n int) (*service.ShareService, *share.Root) {
	bServ := mdutils.Bserv()
	store := dssync.MutexWrap(ds.NewMapDatastore())
	ca := NewShareAvailability(
		light.TestLightAvailability(bServ),
		store,
	)
	return service.NewShareService(bServ, ca), availability_test.RandFillBS(t, n, bServ)
}

// RandFullLocalServiceWithSquare is the same as randFullServiceWithSquare, except
// the share.Availability is wrapped with cache availability.
func RandFullLocalServiceWithSquare(t *testing.T, n int) (*service.ShareService, *share.Root) {
	bServ := mdutils.Bserv()
	store := dssync.MutexWrap(ds.NewMapDatastore())
	ca := NewShareAvailability(
		full.TestAvailability(bServ),
		store,
	)
	return service.NewShareService(bServ, ca), availability_test.RandFillBS(t, n, bServ)
}
