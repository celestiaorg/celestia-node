package share

import (
	"bytes"
	"testing"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/stretchr/testify/require"
)

func TestRowsWithNamespace(t *testing.T) {
	ns1 := testNamespace(1)
	ns2 := testNamespace(2)
	ns3 := testNamespace(3)
	ns4 := testNamespace(4)
	ns5 := testNamespace(5)

	root := &AxisRoots{
		RowRoots: [][]byte{
			testRowRoot(ns1, ns1),
			testRowRoot(ns2, ns3),
			testRowRoot(ns4, ns5),
		},
	}

	rowIdxs, err := RowsWithNamespace(root, ns2)
	require.NoError(t, err)
	require.Equal(t, []int{1}, rowIdxs)

	rowIdxs, err = RowsWithNamespace(root, ns3)
	require.NoError(t, err)
	require.Equal(t, []int{1}, rowIdxs)

	rowIdxs, err = RowsWithNamespace(root, testNamespace(6))
	require.NoError(t, err)
	require.Empty(t, rowIdxs)
}

func TestRowsWithNamespaceReturnsHashErrors(t *testing.T) {
	root := &AxisRoots{
		RowRoots: [][]byte{
			testRowRoot(testNamespace(1), testNamespace(2)),
			[]byte("short"),
		},
	}

	rowIdxs, err := RowsWithNamespace(root, testNamespace(1))
	require.Error(t, err)
	require.Nil(t, rowIdxs)
}

func testNamespace(id byte) libshare.Namespace {
	return libshare.MustNewV0Namespace(bytes.Repeat([]byte{id}, libshare.NamespaceVersionZeroIDSize))
}

func testRowRoot(minNamespace, maxNamespace libshare.Namespace) []byte {
	root := make([]byte, AxisRootSize)
	copy(root[:libshare.NamespaceSize], minNamespace.Bytes())
	copy(root[libshare.NamespaceSize:2*libshare.NamespaceSize], maxNamespace.Bytes())
	return root
}
