package share

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"
)

// TestCacheAvailability tests to ensure that the successful result of a
// sampling process is properly stored locally.
func TestCacheAvailability(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fullLocalServ, dah0 := RandFullLocalServiceWithSquare(t, 16)
	lightLocalServ, dah1 := RandLightLocalServiceWithSquare(t, 16)

	var tests = []struct {
		service *Service
		root    *Root
	}{
		{
			service: fullLocalServ,
			root:    dah0,
		},
		{
			service: lightLocalServ,
			root:    dah1,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ca := tt.service.Availability.(*CacheAvailability)
			// ensure the dah isn't yet in the cache
			exists, err := ca.ds.Has(rootKey(tt.root))
			require.NoError(t, err)
			assert.False(t, exists)
			err = tt.service.SharesAvailable(ctx, tt.root)
			require.NoError(t, err)
			// ensure the dah was stored properly
			exists, err = ca.ds.Has(rootKey(tt.root))
			require.NoError(t, err)
			assert.True(t, exists)
		})
	}
}

var invalidHeader = da.DataAvailabilityHeader{
	RowsRoots: [][]byte{{1, 2}},
}

// TestCacheAvailability_Failed tests to make sure a failed
// sampling process is not stored.
func TestCacheAvailability_Failed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ca := NewCacheAvailability(&dummyAvailability{}, sync.MutexWrap(datastore.NewMapDatastore()))
	serv := NewService(mdutils.Bserv(), ca)

	err := serv.SharesAvailable(ctx, &invalidHeader)
	require.Error(t, err)
	// ensure the dah was NOT cached
	exists, err := ca.ds.Has(rootKey(&invalidHeader))
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestCacheAvailability_NoDuplicateSampling tests to ensure that
// if the Root was already sampled, it does not run a sampling routine
// again.
func TestCacheAvailability_NoDuplicateSampling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// create root to cache
	root := RandFillBS(t, 16, mdutils.Bserv())
	// wrap dummyAvailability with a datastore
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	ca := NewCacheAvailability(&dummyAvailability{counter: 0}, ds)
	// sample the root
	err := ca.SharesAvailable(ctx, root)
	require.NoError(t, err)
	// ensure root was cached
	exists, err := ca.ds.Has(rootKey(root))
	require.NoError(t, err)
	assert.True(t, exists)
	// call sampling routine over same root again and ensure no error is returned
	// if an error was returned, that means duplicate sampling occurred
	err = ca.SharesAvailable(ctx, root)
	require.NoError(t, err)
}

// TestCacheAvailability_MinRoot tests to make sure `SharesAvailable` will
// short circuit if the given root is a minimum DataAvailabilityHeader (minRoot).
func TestCacheAvailability_MinRoot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fullLocalServ, _ := RandFullLocalServiceWithSquare(t, 16)
	minDAH := da.MinDataAvailabilityHeader()

	err := fullLocalServ.SharesAvailable(ctx, &minDAH)
	assert.NoError(t, err)
}

type dummyAvailability struct {
	counter int
}

// SharesAvailable should only be called once, if called more than once, return
// error.
func (da *dummyAvailability) SharesAvailable(_ context.Context, root *Root) error {
	if root == &invalidHeader {
		return fmt.Errorf("invalid header")
	}
	if da.counter > 0 {
		return fmt.Errorf("duplicate sampling process called")
	}
	da.counter++
	return nil
}
