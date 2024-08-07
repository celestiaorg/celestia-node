package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		expectErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				IP:       "127.0.0.1",
				RPCPort:  DefaultRPCPort,
				GRPCPort: DefaultGRPCPort,
			},
			expectErr: false,
		},
		{
			name:      "empty config, no endpoint",
			cfg:       Config{},
			expectErr: false,
		},
		{
			name: "hostname preserved",
			cfg: Config{
				IP:       "celestia.org",
				RPCPort:  DefaultRPCPort,
				GRPCPort: DefaultGRPCPort,
			},
			expectErr: false,
		},
		{
			name: "missing RPC port",
			cfg: Config{
				IP:       "127.0.0.1",
				GRPCPort: DefaultGRPCPort,
			},
			expectErr: true,
		},
		{
			name: "missing GRPC port",
			cfg: Config{
				IP:      "127.0.0.1",
				RPCPort: DefaultRPCPort,
			},
			expectErr: true,
		},
		{
			name: "invalid IP, but will be accepted as host and not raise an error",
			cfg: Config{
				IP:       "invalid-ip",
				RPCPort:  DefaultRPCPort,
				GRPCPort: DefaultGRPCPort,
			},
			expectErr: false,
		},
		{
			name: "invalid RPC port",
			cfg: Config{
				IP:       "127.0.0.1",
				RPCPort:  "invalid-port",
				GRPCPort: DefaultGRPCPort,
			},
			expectErr: true,
		},
		{
			name: "invalid GRPC port",
			cfg: Config{
				IP:       "127.0.0.1",
				RPCPort:  DefaultRPCPort,
				GRPCPort: "invalid-port",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
