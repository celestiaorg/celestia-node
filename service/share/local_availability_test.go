package share

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
)

// TestLocalAvailability tests to ensure that the successful result of a
// sampling process is properly stored locally.
func TestLocalAvailability(t *testing.T) {
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
			la := tt.service.Availability.(*localAvailability)
			// ensure the dah isn't yet in the cache
			exists, err := la.ds.Has(rootKey(tt.root))
			require.NoError(t, err)
			assert.False(t, exists)
			err = tt.service.SharesAvailable(ctx, tt.root)
			require.NoError(t, err)
			// ensure the dah was stored properly
			exists, err = la.ds.Has(rootKey(tt.root))
			require.NoError(t, err)
			assert.True(t, exists)
		})
	}
}

// TestLocalAvailability_Failed tests to make sure a failed
// sampling process is not stored.
func TestLocalAvailability_Failed(t *testing.T) {
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
			la := tt.service.Availability.(*localAvailability)

			empty := header.EmptyDAH()
			err := tt.service.SharesAvailable(ctx, &empty)
			require.Error(t, err)
			// ensure the dah was NOT cached
			exists, err := la.ds.Has(rootKey(&empty))
			require.NoError(t, err)
			assert.False(t, exists)
		})
	}
}

// TestLocalAvailability_NoDuplicateSampling tests to ensure that
// if the Root was already sampled, it does not run a sampling routine
// again.
func TestLocalAvailability_NoDuplicateSampling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// create root to cache
	root := RandFillDAG(t, 16, mdutils.Bserv())
	// wrap dummyAvailability with a datastore
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	la, err := NewLocalAvailability(&dummyAvailability{counter: 0}, ds)
	require.NoError(t, err)
	// sample the root
	err = la.SharesAvailable(ctx, root)
	require.NoError(t, err)
	// ensure root was cached
	exists, err := la.(*localAvailability).ds.Has(rootKey(root))
	require.NoError(t, err)
	assert.True(t, exists)
	// call sampling routine over same root again and ensure no error is returned
	// if an error was returned, that means duplicate sampling occurred
	err = la.SharesAvailable(ctx, root)
	require.NoError(t, err)
}

type dummyAvailability struct {
	counter int
}

// SharesAvailable should only be called once, if called more than once, return
// error.
func (da *dummyAvailability) SharesAvailable(context.Context, *Root) error {
	if da.counter > 0 {
		return fmt.Errorf("duplicate sampling process called")
	}
	da.counter++
	return nil
}
