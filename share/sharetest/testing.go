package sharetest

import (
	"bytes"
	"math/rand"
	"sort"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/namespace"

	"github.com/celestiaorg/celestia-node/share"
)

// RandShares generate 'total' amount of shares filled with random data. It uses require.TestingT
// to be able to take both a *testing.T and a *testing.B.
func RandShares(t require.TestingT, total int) []share.Share {
	if total&(total-1) != 0 {
		t.Errorf("total must be power of 2: %d", total)
		t.FailNow()
	}

	shares := make([]share.Share, total)
	for i := range shares {
		shr := make([]byte, share.Size)
		copy(shr[:share.NamespaceSize], RandNamespace())
		_, err := r.Read(shr[share.NamespaceSize:])
		require.NoError(t, err)
		shares[i] = shr
	}
	sort.Slice(shares, func(i, j int) bool { return bytes.Compare(shares[i], shares[j]) < 0 })

	return shares
}

// RandNamespace generates random valid data namespace for testing purposes.
func RandNamespace() share.Namespace {
	rb := make([]byte, namespace.NamespaceVersionZeroIDSize)
	r.Read(rb)
	for {
		namespace, _ := share.NewNamespaceV0(rb)
		if err := namespace.ValidateDataNamespace(); err != nil {
			continue
		}
		return namespace
	}
}

var r = rand.New(rand.NewSource(time.Now().Unix())) //nolint:gosec
