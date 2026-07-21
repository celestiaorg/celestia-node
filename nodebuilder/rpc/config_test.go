package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc"
)

const testAddr = "127.0.0.1"

// TestDefaultConfig tests that the default rpc config is correct.
func TestDefaultConfig(t *testing.T) {
	expected := Config{
		Address:  defaultBindAddress,
		Port:     defaultPort,
		SkipAuth: false,
		CORS: CORSConfig{
			Enabled:        false,
			AllowedOrigins: []string{},
			AllowedHeaders: []string{},
			AllowedMethods: []string{},
		},
		RateLimit:          DefaultRateLimitConfig(),
		MaxConcurrentConns: rpc.DefaultMaxConcurrentConns,
	}

	assert.Equal(t, expected, DefaultConfig())
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		err  bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Address:            testAddr,
				Port:               "8080",
				MaxConcurrentConns: rpc.DefaultMaxConcurrentConns,
			},
			err: false,
		},
		{
			name: "invalid address",
			cfg: Config{
				Address:            "999.999.999.999",
				Port:               "8080",
				MaxConcurrentConns: rpc.DefaultMaxConcurrentConns,
			},
			err: true,
		},
		{
			name: "invalid port",
			cfg: Config{
				Address:            testAddr,
				Port:               "invalid",
				MaxConcurrentConns: rpc.DefaultMaxConcurrentConns,
			},
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.err {
				t.Errorf("Config.Validate() error = %v, err %v", err, tt.err)
			}
		})
	}
}

// TestConfigValidate_MaxConcurrentConnsFallback covers the upgrade path where
// an older config.toml lacks the field: it decodes to 0, and Validate must
// backfill the default instead of failing the node's startup.
func TestConfigValidate_MaxConcurrentConnsFallback(t *testing.T) {
	tests := []struct {
		name string
		in   int
	}{
		{name: "zero (missing from old config.toml)", in: 0},
		{name: "negative", in: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Address:            testAddr,
				Port:               "8080",
				MaxConcurrentConns: tt.in,
			}
			require.NoError(t, cfg.Validate())
			assert.Equal(t, rpc.DefaultMaxConcurrentConns, cfg.MaxConcurrentConns)
		})
	}
}
