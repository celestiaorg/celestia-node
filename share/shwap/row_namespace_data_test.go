package shwap_test

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestNamespacedRowFromShares(t *testing.T) {
	const odsSize = 8

	minNamespace, err := share.NewBlobNamespaceV0(slices.Concat(bytes.Repeat([]byte{0}, 8), []byte{1, 0}))
	require.NoError(t, err)
	err = minNamespace.ValidateForData()
	require.NoError(t, err)

	for namespacedAmount := 1; namespacedAmount < odsSize; namespacedAmount++ {
		shares := sharetest.RandSharesWithNamespace(t, minNamespace, namespacedAmount, odsSize)
		parity, err := share.DefaultRSMT2DCodec().Encode(shares)
		require.NoError(t, err)
		extended := slices.Concat(shares, parity)

		nr, err := shwap.RowNamespaceDataFromShares(extended, minNamespace, 0)
		require.NoError(t, err)
		require.Equal(t, namespacedAmount, len(nr.Shares))
	}
}

func TestNamespacedRowFromSharesNonIncluded(t *testing.T) {
	// TODO: this will fail until absence proof support is added
	t.Skip()

	const odsSize = 8
	// Test absent namespace
	shares := sharetest.RandShares(t, odsSize)
	absentNs, err := share.GetNamespace(shares[0]).AddInt(1)
	require.NoError(t, err)

	parity, err := share.DefaultRSMT2DCodec().Encode(shares)
	require.NoError(t, err)
	extended := slices.Concat(shares, parity)

	nr, err := shwap.RowNamespaceDataFromShares(extended, absentNs, 0)
	require.NoError(t, err)
	require.Len(t, nr.Shares, 0)
	require.True(t, nr.Proof.IsOfAbsence())
}

func TestValidateNamespacedRow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	const odsSize = 8
	sharesAmount := odsSize * odsSize
	namespace := sharetest.RandV0Namespace()
	for amount := 1; amount < sharesAmount; amount++ {
		randEDS, root := edstest.RandEDSWithNamespace(t, namespace, amount, odsSize)
		rsmt2d := &eds.Rsmt2D{ExtendedDataSquare: randEDS}
		nd, err := eds.NamespacedData(ctx, root, rsmt2d, namespace)
		require.NoError(t, err)
		require.True(t, len(nd) > 0)

		rowIdxs := share.RowsWithNamespace(root, namespace)
		require.Len(t, nd, len(rowIdxs))

		for i, rowIdx := range rowIdxs {
			err = nd[i].Validate(root, namespace, rowIdx)
			require.NoError(t, err)
		}
	}
}

func TestNamespacedRowProtoEncoding(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	const odsSize = 8
	namespace := sharetest.RandV0Namespace()
	randEDS, root := edstest.RandEDSWithNamespace(t, namespace, odsSize, odsSize)
	rsmt2d := &eds.Rsmt2D{ExtendedDataSquare: randEDS}
	nd, err := eds.NamespacedData(ctx, root, rsmt2d, namespace)
	require.NoError(t, err)
	require.True(t, len(nd) > 0)

	expected := nd[0]
	pb := expected.ToProto()
	ndOut := shwap.RowNamespaceDataFromProto(pb)
	require.Equal(t, expected, ndOut)
}
