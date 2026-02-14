//go:build !race

package nodebuilder

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func TestRepo(t *testing.T) {
	tests := []struct {
		tp node.Type
	}{
		{tp: node.Bridge}, {tp: node.Light},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dir := t.TempDir()

			_, err := OpenStore(dir, nil)
			assert.ErrorIs(t, err, ErrNotInited)

			err = Init(*DefaultConfig(tt.tp), dir, tt.tp)
			require.NoError(t, err)

			store, err := OpenStore(dir, nil)
			require.NoError(t, err)

			_, err = OpenStore(dir, nil)
			assert.ErrorIs(t, err, ErrOpened)

			ks, err := store.Keystore()
			assert.NoError(t, err)
			assert.NotNil(t, ks)

			data, err := store.Datastore()
			assert.NoError(t, err)
			assert.NotNil(t, data)

			cfg, err := store.Config()
			assert.NoError(t, err)
			assert.NotNil(t, cfg)

			err = store.Close()
			assert.NoError(t, err)
		})
	}
}

func TestDiscoverOpened(t *testing.T) {
	t.Run("single open store", func(t *testing.T) {
		_, dir := initAndOpenStore(t, node.Bridge)

		mockDefaultNodeStorePath := func(t node.Type, n p2p.Network) (string, error) {
			return dir, nil
		}
		DefaultNodeStorePath = mockDefaultNodeStorePath

		path, err := DiscoverOpened()
		require.NoError(t, err)
		require.Equal(t, dir, path)
	})

	t.Run("multiple open nodes by preference order", func(t *testing.T) {
		networks := []p2p.Network{p2p.Mainnet, p2p.Mocha, p2p.Arabica, p2p.Private}
		nodeTypes := []node.Type{node.Bridge, node.Light}

		// Store opened stores in a map (network + node -> dir/store)
		dirMap := make(map[string]string)
		storeMap := make(map[string]Store)
		for _, network := range networks {
			for _, tp := range nodeTypes {
				store, dir := initAndOpenStore(t, tp)
				key := network.String() + "_" + tp.String()
				dirMap[key] = dir
				storeMap[key] = store
			}
		}

		mockDefaultNodeStorePath := func(tp node.Type, n p2p.Network) (string, error) {
			key := n.String() + "_" + tp.String()
			if dir, ok := dirMap[key]; ok {
				return dir, nil
			}
			return "", fmt.Errorf("no store for %s_%s", n, tp)
		}
		DefaultNodeStorePath = mockDefaultNodeStorePath

		// Discover opened stores in preference order
		for _, network := range networks {
			for _, tp := range nodeTypes {
				path, err := DiscoverOpened()
				require.NoError(t, err)
				key := network.String() + "_" + tp.String()
				require.Equal(t, dirMap[key], path)

				// close the store to discover the next one
				storeMap[key].Close()
			}
		}
	})

	t.Run("no opened store", func(t *testing.T) {
		dir := t.TempDir()
		mockDefaultNodeStorePath := func(t node.Type, n p2p.Network) (string, error) {
			return dir, nil
		}
		DefaultNodeStorePath = mockDefaultNodeStorePath

		path, err := DiscoverOpened()
		assert.ErrorIs(t, err, ErrNoOpenStore)
		assert.Empty(t, path)
	})
}

func TestIsOpened(t *testing.T) {
	dir := t.TempDir()

	// Case 1: non-existent node store
	ok, err := IsOpened(dir)
	require.NoError(t, err)
	require.False(t, ok)

	// Case 2: initialized node store, not locked
	err = Init(*DefaultConfig(node.Bridge), dir, node.Bridge)
	require.NoError(t, err)
	ok, err = IsOpened(dir)
	require.NoError(t, err)
	require.False(t, ok)

	// Case 3: initialized node store, locked
	flk := flock.New(lockPath(dir))
	_, err = flk.TryLock()
	require.NoError(t, err)
	defer flk.Unlock() //nolint:errcheck
	ok, err = IsOpened(dir)
	require.NoError(t, err)
	require.True(t, ok)
}

func initAndOpenStore(t *testing.T, tp node.Type) (store Store, dir string) {
	dir = t.TempDir()
	err := Init(*DefaultConfig(tp), dir, tp)
	require.NoError(t, err)
	store, err = OpenStore(dir, nil)
	require.NoError(t, err)
	return store, dir
}
