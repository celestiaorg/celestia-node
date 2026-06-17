package share_test

import (
	"testing"

	nodeshare "github.com/celestiaorg/celestia-node/share"
	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/stretchr/testify/require"
)

func TestRowsWithNamespace(t *testing.T) {
	namespace := testNamespace(t, 0x10)

	root := &nodeshare.AxisRoots{
		RowRoots: [][]byte{
			testRowRoot(t, testNamespace(t, 0x00), testNamespace(t, 0x09)),
			testRowRoot(t, namespace, namespace),
			testRowRoot(t, testNamespace(t, 0x0f), testNamespace(t, 0x11)),
			testRowRoot(t, testNamespace(t, 0x12), testNamespace(t, 0x20)),
		},
	}

	rows, err := nodeshare.RowsWithNamespace(root, namespace)

	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, rows)
}

func TestRowsWithNamespaceReturnsEmptyWhenNamespaceIsOutsideRows(t *testing.T) {
	namespace := testNamespace(t, 0x10)
	root := &nodeshare.AxisRoots{
		RowRoots: [][]byte{
			testRowRoot(t, testNamespace(t, 0x00), testNamespace(t, 0x09)),
			testRowRoot(t, testNamespace(t, 0x12), testNamespace(t, 0x20)),
		},
	}

	rows, err := nodeshare.RowsWithNamespace(root, namespace)

	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestRowsWithNamespaceReturnsErrorForInvalidRowRoot(t *testing.T) {
	root := &nodeshare.AxisRoots{
		RowRoots: [][]byte{
			make([]byte, libshare.NamespaceSize),
		},
	}

	rows, err := nodeshare.RowsWithNamespace(root, testNamespace(t, 0x10))

	require.Nil(t, rows)
	require.ErrorContains(t, err, "rightHash can't be less than")
}

func testNamespace(t *testing.T, value byte) libshare.Namespace {
	t.Helper()

	namespace, err := libshare.NewV0Namespace([]byte{value})
	require.NoError(t, err)
	return namespace
}

func testRowRoot(t *testing.T, min, max libshare.Namespace) []byte {
	t.Helper()
	require.True(t, min.IsLessOrEqualThan(max))

	root := make([]byte, nodeshare.AxisRootSize)
	copy(root[:libshare.NamespaceSize], min.Bytes())
	copy(root[libshare.NamespaceSize:2*libshare.NamespaceSize], max.Bytes())
	return root
}
