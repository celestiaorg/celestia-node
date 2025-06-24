package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultConfig tests that the default gateway config is correct.
func TestDefaultConfig(t *testing.T) {
	expected := Config{
		Address: defaultBindAddress,
		Port:    defaultPort,
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
				Address: "127.0.0.1",
				Port:    "8080",
			},
			err: false,
		},
		{
			name: "invalid address",
			cfg: Config{
				Address: "invalid",
				Port:    "8080",
			},
			err: true,
		},
		{
			name: "invalid port",
			cfg: Config{
				Address: "127.0.0.1",
				Port:    "invalid",
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

func TestRequestURL(t *testing.T) {
	t.Run("http default", func(t *testing.T) {
		cfg := Config{Address: "127.0.0.1", Port: "8080", TLSEnabled: false}
		assert.Equal(t, "http://127.0.0.1:8080", cfg.RequestURL())
	})
	t.Run("https enabled", func(t *testing.T) {
		cfg := Config{Address: "127.0.0.1", Port: "8080", TLSEnabled: true}
		assert.Equal(t, "https://127.0.0.1:8080", cfg.RequestURL())
	})
	t.Run("address with http prefix", func(t *testing.T) {
		cfg := Config{Address: "http://myhost", Port: "1234", TLSEnabled: false}
		assert.Equal(t, "http://myhost:1234", cfg.RequestURL())
	})
	t.Run("address with https prefix", func(t *testing.T) {
		cfg := Config{Address: "https://myhost", Port: "1234", TLSEnabled: true}
		assert.Equal(t, "https://myhost:1234", cfg.RequestURL())
	})
}
