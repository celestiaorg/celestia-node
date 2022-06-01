package share

import (
	"context"
	"fmt"
	"testing"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
)

// TestSharesAvailableInCache tests to ensure that the successful result of a
// sampling process is properly cached.
func TestSharesAvailableInCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, dah := RandFullCacheServiceWithSquare(t, 16)
	ca := service.Availability.(*cacheAvailability)
	// ensure the dah isn't yet in the cache
	_, ok := ca.cache.Get(dah.String())
	assert.False(t, ok)
	err := service.SharesAvailable(ctx, dah)
	require.NoError(t, err)
	// ensure the dah was cached
	_, ok = ca.cache.Get(dah.String())
	assert.True(t, ok)
}

// TestSharesAvailableInCache_Failed tests to make sure a failed
// sampling process is not cached.
func TestSharesAvailableInCache_Failed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, _ := RandFullCacheServiceWithSquare(t, 16)
	ca := service.Availability.(*cacheAvailability)

	empty := header.EmptyDAH()
	err := service.SharesAvailable(ctx, &empty)
	require.Error(t, err)
	// ensure the dah was NOT cached
	_, ok := ca.cache.Get(empty.String())
	assert.False(t, ok)
}

// TestSharesAvailableInCache_NoDuplicateSampling tests to ensure that
// if the Root was already sampled, it does not run a sampling routine
// again.
func TestSharesAvailableInCache_NoDuplicateSampling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// create root to cache
	root := RandFillDAG(t, 16, mdutils.Bserv())
	// wrap dummyAvailability with cache
	ca, err := NewCacheAvailability(&dummyAvailability{counter: 0})
	require.NoError(t, err)
	// sample the root
	err = ca.SharesAvailable(ctx, root)
	require.NoError(t, err)
	// ensure root was cached
	_, ok := ca.(*cacheAvailability).cache.Get(root.String())
	assert.True(t, ok)
	// call sampling routine over same root again and ensure no error is returned
	// if an error was returned, that means duplicate sampling occurred
	err = ca.SharesAvailable(ctx, root)
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

func (da *dummyAvailability) Stop(context.Context) error { return nil }
