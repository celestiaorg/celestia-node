package sharetest

import (
	"bytes"
	"math/rand"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-square/namespace"

	"github.com/celestiaorg/celestia-node/share"
)

// RandShares generate 'total' amount of shares filled with random data.
func RandShares(t testing.TB, total int) []share.Share {
	if total&(total-1) != 0 {
		t.Errorf("total must be power of 2: %d", total)
		t.FailNow()
	}

	shares := make([]share.Share, total)
	for i := range shares {
		shr := make([]byte, share.Size)
		copy(share.GetNamespace(shr), RandV0Namespace())
		rndMu.Lock()
		_, err := rnd.Read(share.GetData(shr))
		rndMu.Unlock()
		require.NoError(t, err)
		shares[i] = shr
	}
	sort.Slice(shares, func(i, j int) bool { return bytes.Compare(shares[i], shares[j]) < 0 })

	return shares
}

// RandSharesWithNamespace is same the as RandShares, but sets same namespace for all shares.
func RandSharesWithNamespace(t testing.TB, namespace share.Namespace, namespacedAmount, total int) []share.Share {
	if total&(total-1) != 0 {
		t.Errorf("total must be power of 2: %d", total)
		t.FailNow()
	}

	if namespacedAmount > total {
		t.Errorf("withNamespace must be less than total: %d", total)
		t.FailNow()
	}

	shares := make([]share.Share, total)
	rnd := rand.New(rand.NewSource(time.Now().Unix())) //nolint:gosec
	for i := range shares {
		shr := make([]byte, share.Size)
		if i < namespacedAmount {
			copy(share.GetNamespace(shr), namespace)
		} else {
			copy(share.GetNamespace(shr), RandV0Namespace())
		}
		_, err := rnd.Read(share.GetData(shr))
		require.NoError(t, err)
		shares[i] = shr
	}
	sort.Slice(shares, func(i, j int) bool { return bytes.Compare(shares[i], shares[j]) < 0 })
	return shares
}

// RandV0Namespace generates random valid data namespace for testing purposes.
func RandV0Namespace() share.Namespace {
	rb := make([]byte, namespace.NamespaceVersionZeroIDSize)
	rndMu.Lock()
	rnd.Read(rb)
	rndMu.Unlock()
	for {
		namespace, _ := share.NewBlobNamespaceV0(rb)
		if err := namespace.ValidateForData(); err != nil {
			continue
		}
		return namespace
	}
}

var (
	rnd   = rand.New(rand.NewSource(time.Now().Unix())) //nolint:gosec
	rndMu sync.Mutex
)
