package gateway

import (
	"bytes"
	"fmt"
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
			NamespaceID:      ns.Bytes()[1:],
			NamespaceVersion: namespace.NamespaceVersionZero,
			ShareVersion:     appconsts.ShareVersionZero,
		}
		err := sss.Write(b)
		require.NoError(t, err)
	}

	sssShares := sss.Export()

	fmt.Println("exported shares: ", sssShares)

	rawSSSShares := make([][]byte, len(sssShares))
	for i := 0; i < len(sssShares); i++ {
		d := sssShares[i].ToBytes()
		rawSSSShares[i] = d
	}

	parsedSSSShares, err := dataFromShares(rawSSSShares)
	require.NoError(t, err)

	require.Equal(t, input, parsedSSSShares)
}

// padShare returns a share padded with trailing zeros.
func padShare(share []byte) (paddedShare []byte) {
	return fillShare(share, 0)
}

// fillShare returns a share filled with filler so that the share length
// is equal to appconsts.ShareSize.
func fillShare(share []byte, filler byte) (paddedShare []byte) {
	return append(share, bytes.Repeat([]byte{filler}, appconsts.ShareSize-len(share))...)
}
