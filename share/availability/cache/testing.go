package cache

import (
	"testing"

	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/service"
)

// RandLightLocalServiceWithSquare is the same as light.RandServiceWithSquare, except
// the share.Availability is wrapped with cache availability.
func RandLightLocalServiceWithSquare(t *testing.T, n int) (*service.ShareService, *share.Root, error) {
	bServ := mdutils.Bserv()
	store := dssync.MutexWrap(ds.NewMapDatastore())
	la, err := light.TestAvailability(bServ)
	if err != nil {
		return nil, nil, err
	}
	avail := NewShareAvailability(
		la,
		store,
	)
	return service.NewShareService(bServ, avail), availability_test.RandFillBS(t, n, bServ), nil
}

// RandFullLocalServiceWithSquare is the same as full.RandServiceWithSquare, except
// the share.Availability is wrapped with cache availability.
func RandFullLocalServiceWithSquare(t *testing.T, n int) (*service.ShareService, *share.Root) {
	bServ := mdutils.Bserv()
	store := dssync.MutexWrap(ds.NewMapDatastore())
	avail := NewShareAvailability(
		full.TestAvailability(bServ),
		store,
	)
	return service.NewShareService(bServ, avail), availability_test.RandFillBS(t, n, bServ)
}
