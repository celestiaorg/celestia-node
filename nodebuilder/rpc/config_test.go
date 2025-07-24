package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
