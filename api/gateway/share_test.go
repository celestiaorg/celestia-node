package gateway

import (
	"testing"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/types"
)

func Test_dataFromShares(t *testing.T) {
	input := [][]byte{
		[]byte("beep"),
		[]byte("beeap"),
		[]byte("BEEEEAHP"),
	}

	ns := namespace.RandomBlobNamespace()
	sss := shares.NewSparseShareSplitter()
	for _, i := range input {
		b := coretypes.Blob{
			Data:             i,
			NamespaceID:      ns.ID,
			NamespaceVersion: ns.Version,
			ShareVersion:     appconsts.ShareVersionZero,
		}
		err := sss.Write(b)
		require.NoError(t, err)
	}

	sssShares := sss.Export()

	rawSSSShares := make([][]byte, len(sssShares))
	for i := 0; i < len(sssShares); i++ {
		d := sssShares[i].ToBytes()
		rawSSSShares[i] = d
	}

	parsedSSSShares, err := dataFromShares(rawSSSShares)
	require.NoError(t, err)

	require.Equal(t, input, parsedSSSShares)
}
