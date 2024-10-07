package gateway

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"

	gosquare "github.com/celestiaorg/go-square/v2/share"
)

func Test_dataFromShares(t *testing.T) {
	testData := [][]byte{
		[]byte("beep"),
		[]byte("beeap"),
		[]byte("BEEEEAHP"),
	}

	ns := gosquare.RandomNamespace()
	sss := gosquare.NewSparseShareSplitter()
	for _, data := range testData {
		b, err := gosquare.NewBlob(ns, data, gosquare.ShareVersionZero, nil)
		require.NoError(t, err)
		require.NoError(t, sss.Write(b))
	}

	sssShares := sss.Export()

	rawSSSShares := make([][]byte, len(sssShares))
	for i := 0; i < len(sssShares); i++ {
		d := sssShares[i].ToBytes()
		rawSSSShares[i] = d
	}

	shrs, err := gosquare.FromBytes(rawSSSShares)
	require.NoError(t, err)
	parsedSSSShares, err := dataFromShares(shrs)
	require.NoError(t, err)

	require.Equal(t, testData, parsedSSSShares)
}
