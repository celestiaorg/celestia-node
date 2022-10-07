package ipld

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/pkg/consts"
)

func TestNamespaceHasherWrite(t *testing.T) {
	leafSize := consts.ShareSize + consts.NamespaceSize
	innerSize := nmtHashSize * 2
	tt := []struct {
		name         string
		expectedSize int
		writtenSize  int
	}{
		{
			"Leaf",
			leafSize,
			leafSize,
		},
		{
			"Inner",
			innerSize,
			innerSize,
		},
		{
			"LeafAndType",
			leafSize,
			leafSize + typeSize,
		},
		{
			"InnerAndType",
			innerSize,
			innerSize + typeSize,
		},
	}

	for _, ts := range tt {
		t.Run("Success"+ts.name, func(t *testing.T) {
			h := defaultHasher()
			n, err := h.Write(make([]byte, ts.writtenSize))
			assert.NoError(t, err)
			assert.Equal(t, ts.expectedSize, n)
			assert.Equal(t, ts.expectedSize, len(h.data))
		})
	}

	t.Run("ErrorSecondWrite", func(t *testing.T) {
		h := defaultHasher()
		n, err := h.Write(make([]byte, leafSize))
		assert.NoError(t, err)
		assert.Equal(t, leafSize, n)

		n, err = h.Write(make([]byte, leafSize))
		assert.Error(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("ErrorIncorrectSize", func(t *testing.T) {
		h := defaultHasher()
		n, err := h.Write(make([]byte, 13))
		assert.Error(t, err)
		assert.Equal(t, 0, n)
	})
}
