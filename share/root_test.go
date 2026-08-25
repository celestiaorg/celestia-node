package share

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
)

func TestRowsWithNamespace(t *testing.T) {
	root := &AxisRoots{
		RowRoots: [][]byte{
			testRowRoot(testNamespace(1), testNamespace(2)),
			testRowRoot(testNamespace(2), testNamespace(4)),
			testRowRoot(testNamespace(5), testNamespace(5)),
		},
	}

	tests := []struct {
		name      string
		namespace libshare.Namespace
		root      *AxisRoots
		want      []int
	}{
		{name: "below every row", namespace: testNamespace(0), root: root},
		{name: "shared boundary", namespace: testNamespace(2), root: root, want: []int{0, 1}},
		{name: "inside range", namespace: testNamespace(3), root: root, want: []int{1}},
		{name: "maximum boundary", namespace: testNamespace(4), root: root, want: []int{1}},
		{name: "single namespace range", namespace: testNamespace(5), root: root, want: []int{2}},
		{name: "above every row", namespace: testNamespace(6), root: root},
		{name: "empty roots", namespace: testNamespace(1), root: &AxisRoots{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := RowsWithNamespace(tt.root, tt.namespace)
			require.NoError(t, err)
			require.Equal(t, tt.want, rows)
		})
	}
}

func TestRowsWithNamespace_InvalidRowRoot(t *testing.T) {
	tests := []struct {
		name string
		root []byte
	}{
		{name: "missing minimum namespace", root: make([]byte, libshare.NamespaceSize-1)},
		{name: "missing maximum namespace", root: make([]byte, libshare.NamespaceSize)},
		{name: "truncated maximum namespace", root: make([]byte, 2*libshare.NamespaceSize-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &AxisRoots{
				RowRoots: [][]byte{
					testRowRoot(testNamespace(1), testNamespace(2)),
					tt.root,
				},
			}

			rows, err := RowsWithNamespace(root, testNamespace(1))
			require.Error(t, err)
			require.Nil(t, rows)
		})
	}
}

func FuzzRowsWithNamespace(f *testing.F) {
	f.Add(byte(1), byte(1), byte(2), byte(3), byte(4))
	f.Add(byte(2), byte(1), byte(2), byte(2), byte(4))
	f.Add(byte(255), byte(0), byte(254), byte(255), byte(255))

	f.Fuzz(func(t *testing.T, query, firstA, firstB, secondA, secondB byte) {
		firstMin, firstMax := ordered(firstA, firstB)
		secondMin, secondMax := ordered(secondA, secondB)
		root := &AxisRoots{
			RowRoots: [][]byte{
				testRowRoot(testNamespace(firstMin), testNamespace(firstMax)),
				testRowRoot(testNamespace(secondMin), testNamespace(secondMax)),
			},
		}

		rows, err := RowsWithNamespace(root, testNamespace(query))
		require.NoError(t, err)

		var want []int
		if firstMin <= query && query <= firstMax {
			want = append(want, 0)
		}
		if secondMin <= query && query <= secondMax {
			want = append(want, 1)
		}
		require.Equal(t, want, rows)
	})
}

func testNamespace(value byte) libshare.Namespace {
	return libshare.MustNewV0Namespace(bytes.Repeat([]byte{value}, libshare.NamespaceVersionZeroIDSize))
}

func testRowRoot(minNamespace, maxNamespace libshare.Namespace) []byte {
	root := make([]byte, AxisRootSize)
	copy(root[:libshare.NamespaceSize], minNamespace.Bytes())
	copy(root[libshare.NamespaceSize:2*libshare.NamespaceSize], maxNamespace.Bytes())
	return root
}

func ordered(a, b byte) (byte, byte) {
	if a > b {
		return b, a
	}
	return a, b
}
