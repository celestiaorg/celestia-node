package nodebuilder

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestRepo(t *testing.T) {
	var tests = []struct {
		tp node.Type
	}{
		{tp: node.Bridge}, {tp: node.Light}, {tp: node.Full},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dir := t.TempDir()

			_, err := OpenStore(dir)
			assert.ErrorIs(t, err, ErrNotInited)

			err = Init(*DefaultConfig(tt.tp), dir, tt.tp)
			require.NoError(t, err)

			store, err := OpenStore(dir)
			require.NoError(t, err)

			_, err = OpenStore(dir)
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
