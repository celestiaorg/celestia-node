package txclient

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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

// A nil *TxConfig must normalize to defaults instead of panicking. Over
// JSON-RPC a `null` options argument reaches the state methods as nil.
func TestWithDefaults(t *testing.T) {
	t.Run("nil becomes defaults", func(t *testing.T) {
		cfg := WithDefaults(nil)
		require.NotNil(t, cfg)
		assert.Equal(t, DefaultGasPrice, cfg.GasPrice())
		assert.Equal(t, DefaultMaxGasPrice, cfg.MaxGasPrice())
		assert.Empty(t, cfg.SignerAddress())
		assert.Empty(t, cfg.KeyName())
		assert.Empty(t, cfg.FeeGranterAddress())
	})

	t.Run("non-nil passes through unchanged", func(t *testing.T) {
		original := NewTxConfig(WithGasPrice(0.005))
		cfg := WithDefaults(original)
		assert.Same(t, original, cfg)
		assert.Equal(t, 0.005, cfg.GasPrice())
	})
}
