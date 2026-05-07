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
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: DefaultPort,
				},
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
				EndpointConfig: EndpointConfig{
					IP:   "celestia.org",
					Port: DefaultPort,
				},
			},
			expectErr: false,
		},
		{
			name: "missing GRPC port",
			cfg: Config{
				EndpointConfig: EndpointConfig{
					IP: "127.0.0.1",
				},
			},
			expectErr: true,
		},
		{
			name: "invalid IP, but will be accepted as host and not raise an error",
			cfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "invalid-ip",
					Port: DefaultPort,
				},
			},
			expectErr: false,
		},
		{
			name: "invalid port",
			cfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: "invalid-port",
				},
			},
			expectErr: true,
		},
		{
			name: "valid additional endpoints",
			cfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: DefaultPort,
				},
				AdditionalCoreEndpoints: []EndpointConfig{
					{
						IP:   "248.249.255.138",
						Port: "4040",
					},
					{
						IP:   "101.255.7.172",
						Port: DefaultPort,
					},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid additional endpoints",
			cfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: DefaultPort,
				},
				AdditionalCoreEndpoints: []EndpointConfig{
					{
						IP:   "248.249.255.138",
						Port: "invalid-port",
					},
					{
						IP:   "101.255.7.172",
						Port: DefaultPort,
					},
				},
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
