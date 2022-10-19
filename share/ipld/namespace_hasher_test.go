package ipld

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
)

func TestNamespaceHasherWrite(t *testing.T) {
	leafSize := appconsts.ShareSize + appconsts.NamespaceSize
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

func TestNamespaceHasherSum(t *testing.T) {
	leafSize := appconsts.ShareSize + appconsts.NamespaceSize
	innerSize := nmtHashSize * 2
	tt := []struct {
		name         string
		expectedSize int
		writtenSize  int
	}{
		{
			"Leaf",
			nmtHashSize,
			leafSize,
		},
		{
			"Inner",
			nmtHashSize,
			innerSize,
		},
		{
			"ShortGarbage",
			0,
			13,
		},
		{
			"LongGarbage",
			0,
			500,
		},
	}

	for _, ts := range tt {
		t.Run("Success"+ts.name, func(t *testing.T) {
			h := defaultHasher()
			_, _ = h.Write(make([]byte, ts.writtenSize))
			sum := h.Sum(nil)
			assert.Equal(t, len(sum), ts.expectedSize)
		})
	}
}
