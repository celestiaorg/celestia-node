package state

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshallingOptions(t *testing.T) {
	opts := NewTxOptions(
		WithGas(10_000),
		WithGasPrice(0.002),
		WithKeyName("test"),
		WithSignerAddress("celestia1eucs6ax66ypjcmwj81hak531w6hyr2c4g8cfsgc"),
		WithFeeGranterAddress("celestia1hakc56ax66ypjcmwj8w6hyr2c4g8cfs3wesguc"),
	)

	data, err := json.Marshal(opts)
	require.NoError(t, err)

	newOpts := &TxOptions{}
	err = json.Unmarshal(data, newOpts)
	require.NoError(t, err)
	require.Equal(t, opts, newOpts)
}
