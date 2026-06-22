package txclient

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshallingOptions(t *testing.T) {
	opts := NewTxConfig(
		WithGas(10_000),
		WithGasPrice(0.002),
		WithKeyName("test"),
		WithSignerAddress("celestia1eucs6ax66ypjcmwj81hak531w6hyr2c4g8cfsgc"),
		WithFeeGranterAddress("celestia1hakc56ax66ypjcmwj8w6hyr2c4g8cfs3wesguc"),
	)

	data, err := json.Marshal(opts)
	require.NoError(t, err)

	newOpts := &TxConfig{}
	err = json.Unmarshal(data, newOpts)
	require.NoError(t, err)
	require.Equal(t, opts, newOpts)
}

func TestMaxGasPrice(t *testing.T) {
	t.Run("unset returns default", func(t *testing.T) {
		cfg := NewTxConfig()
		require.Equal(t, DefaultMaxGasPrice, cfg.MaxGasPrice())
	})

	t.Run("explicit zero is honored, not replaced by default", func(t *testing.T) {
		cfg := NewTxConfig(WithMaxGasPrice(0))
		require.Equal(t, float64(0), cfg.MaxGasPrice())
	})

	t.Run("explicit non-zero is honored", func(t *testing.T) {
		cfg := NewTxConfig(WithMaxGasPrice(1.5))
		require.Equal(t, 1.5, cfg.MaxGasPrice())
	})

	t.Run("explicit zero survives JSON round-trip", func(t *testing.T) {
		cfg := NewTxConfig(WithMaxGasPrice(0))
		data, err := json.Marshal(cfg)
		require.NoError(t, err)

		out := &TxConfig{}
		require.NoError(t, json.Unmarshal(data, out))
		require.Equal(t, float64(0), out.MaxGasPrice())
	})
}
