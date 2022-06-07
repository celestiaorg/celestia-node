package node

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo(t *testing.T) {
	var tests = []struct {
		tp Type
	}{
		{tp: Bridge}, {tp: Light}, {tp: Full},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dir := t.TempDir()

			_, err := OpenStore(dir)
			assert.ErrorIs(t, err, ErrNotInited)

			err = Init(dir, tt.tp)
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
