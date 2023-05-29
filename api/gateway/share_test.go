package gateway

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/celestiaorg/celestia-app/pkg/shares"
)

func Test_dataFromShares(t *testing.T) {
	testData := [][]byte{
		[]byte("beep"),
		[]byte("beeap"),
		[]byte("BEEEEAHP"),
	}

	ns := namespace.RandomBlobNamespace()
	sss := shares.NewSparseShareSplitter()
	for _, data := range testData {
		b := coretypes.Blob{
			Data:             data,
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

	require.Equal(t, testData, parsedSSSShares)
}

// sharesBase64JSON is the base64 encoded share data from Blockspace Race
// block height 559108 and namespace e8e5f679bf7116cb.
//
//go:embed "testdata/sharesBase64.json"
var sharesBase64JSON string

// Test_dataFromSharesBSR reproduces an error that occurred when parsing shares
// on Blockspace Race block height 559108 namespace e8e5f679bf7116cb.
//
// https://github.com/celestiaorg/celestia-app/issues/1816
func Test_dataFromSharesBSR(t *testing.T) {
	t.Skip("skip until sharesBase64JSON is regenerated with v1 compatibility")

	var sharesBase64 []string
	err := json.Unmarshal([]byte(sharesBase64JSON), &sharesBase64)
	assert.NoError(t, err)
	input := decode(sharesBase64)

	_, err = dataFromShares(input)
	assert.NoError(t, err)
}

// decode returns the raw shares from base64Encoded.
func decode(base64Encoded []string) (rawShares [][]byte) {
	for _, share := range base64Encoded {
		rawShare, err := base64.StdEncoding.DecodeString(share)
		if err != nil {
			panic(err)
		}
		rawShares = append(rawShares, rawShare)
	}
	return rawShares
}
