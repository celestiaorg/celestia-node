package gateway

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"
)

func Test_dataFromShares(t *testing.T) {
	testData := [][]byte{
		[]byte("beep"),
		[]byte("beeap"),
		[]byte("BEEEEAHP"),
	}

	ns := libshare.RandomNamespace()
	sss := libshare.NewSparseShareSplitter()
	for _, data := range testData {
		b, err := libshare.NewBlob(ns, data, libshare.ShareVersionZero, nil)
		require.NoError(t, err)
		require.NoError(t, sss.Write(b))
	}

	sssShares := sss.Export()

	rawSSSShares := make([][]byte, len(sssShares))
	for i := range sssShares {
		d := sssShares[i].ToBytes()
		rawSSSShares[i] = d
	}

	shrs, err := libshare.FromBytes(rawSSSShares)
	require.NoError(t, err)
	parsedSSSShares, err := dataFromShares(shrs)
	require.NoError(t, err)

	require.Equal(t, testData, parsedSSSShares)
}
