package shwap

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ns := sharetest.RandV0Namespace()
	square, root := edstest.RandEDSWithNamespace(t, ns, 16)
	hdr := &header.ExtendedHeader{DAH: root}

	b := edsBlockstore(square)
	bserv := NewBlockService(b, nil)
	get := NewGetter(bserv)

	t.Run("GetShares", func(t *testing.T) {
		idxs := rand.Perm(int(square.Width() ^ 2))[:30]
		shrs, err := get.GetShares(ctx, hdr, idxs...)
		assert.NoError(t, err)

		for i, shrs := range shrs {
			idx := idxs[i]
			x, y := uint(idx)/square.Width(), uint(idx)%square.Width()
			cell := square.GetCell(x, y)
			ok := bytes.Equal(cell, shrs)
			require.True(t, ok)
		}
	})

	t.Run("GetEDS", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		eds, err := get.GetEDS(ctx, hdr)
		assert.NoError(t, err)
		assert.NotNil(t, eds)

		ok := eds.Equals(square)
		assert.True(t, ok)
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		nshrs, err := get.GetSharesByNamespace(ctx, hdr, ns)
		assert.NoError(t, err)
		assert.NoError(t, nshrs.Verify(root, ns))
		assert.NotEmpty(t, nshrs.Flatten())

		t.Run("NamespaceOutsideOfRoot", func(t *testing.T) {
			randNamespace := sharetest.RandV0Namespace()
			emptyShares, err := get.GetSharesByNamespace(ctx, hdr, randNamespace)
			assert.NoError(t, err)
			assert.Empty(t, emptyShares)
			assert.NoError(t, emptyShares.Verify(root, randNamespace))
			assert.Empty(t, emptyShares.Flatten())
		})

		t.Run("NamespaceInsideOfRoot", func(t *testing.T) {
			square := edstest.RandEDS(t, 8)
			root, err := share.NewRoot(square)
			require.NoError(t, err)
			hdr := &header.ExtendedHeader{DAH: root}

			b := edsBlockstore(square)
			bserv := NewBlockService(b, nil)
			get := NewGetter(bserv)

			maxNs := nmt.MaxNamespace(root.RowRoots[(len(root.RowRoots))/2-1], share.NamespaceSize)
			ns, err := addToNamespace(maxNs, -1)
			require.NoError(t, err)
			require.Len(t, ipld.FilterRootByNamespace(root, ns), 1)

			emptyShares, err := get.GetSharesByNamespace(ctx, hdr, ns)
			assert.NoError(t, err)
			assert.NotNil(t, emptyShares[0].Proof)
			assert.NoError(t, emptyShares.Verify(root, ns))
			assert.Empty(t, emptyShares.Flatten())
		})
	})
}

// addToNamespace adds arbitrary int value to namespace, treating namespace as big-endian
// implementation of int
// TODO: dedup with getters/shrex_test.go
func addToNamespace(namespace share.Namespace, val int) (share.Namespace, error) {
	if val == 0 {
		return namespace, nil
	}
	// Convert the input integer to a byte slice and add it to result slice
	result := make([]byte, len(namespace))
	if val > 0 {
		binary.BigEndian.PutUint64(result[len(namespace)-8:], uint64(val))
	} else {
		binary.BigEndian.PutUint64(result[len(namespace)-8:], uint64(-val))
	}

	// Perform addition byte by byte
	var carry int
	for i := len(namespace) - 1; i >= 0; i-- {
		sum := 0
		if val > 0 {
			sum = int(namespace[i]) + int(result[i]) + carry
		} else {
			sum = int(namespace[i]) - int(result[i]) + carry
		}

		switch {
		case sum > 255:
			carry = 1
			sum -= 256
		case sum < 0:
			carry = -1
			sum += 256
		default:
			carry = 0
		}

		result[i] = uint8(sum)
	}

	// Handle any remaining carry
	if carry != 0 {
		return nil, fmt.Errorf("namespace overflow")
	}

	return result, nil
}
