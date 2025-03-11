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
				IP:   "127.0.0.1",
				Port: DefaultPort,
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
				IP:   "celestia.org",
				Port: DefaultPort,
			},
			expectErr: false,
		},
		{
			name: "missing GRPC port",
			cfg: Config{
				IP: "127.0.0.1",
			},
			expectErr: true,
		},
		{
			name: "invalid IP, but will be accepted as host and not raise an error",
			cfg: Config{
				IP:   "invalid-ip",
				Port: DefaultPort,
			},
			expectErr: false,
		},
		{
			name: "invalid port",
			cfg: Config{
				IP:   "127.0.0.1",
				Port: "invalid-port",
			},
			expectErr: true,
		},
		{
			name: "with multiple valid endpoints",
			cfg: Config{
				IP:   "127.0.0.1",
				Port: "9090",
				Endpoints: []Endpoint{
					{
						IP:   "127.0.0.2",
						Port: "9090",
					},
					{
						IP:   "127.0.0.3",
						Port: "9091",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "multiple endpoints with empty legacy endpoint",
			cfg: Config{
				Endpoints: []Endpoint{
					{
						IP:   "127.0.0.2",
						Port: "9090",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "multiple endpoints with invalid port",
			cfg: Config{
				IP:   "127.0.0.1",
				Port: "9090",
				Endpoints: []Endpoint{
					{
						IP:   "127.0.0.2",
						Port: "invalid",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "multiple endpoints with empty IP",
			cfg: Config{
				IP:   "127.0.0.1",
				Port: "9090",
				Endpoints: []Endpoint{
					{
						IP:   "",
						Port: "9090",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "multiple endpoints with empty port",
			cfg: Config{
				IP:   "127.0.0.1",
				Port: "9090",
				Endpoints: []Endpoint{
					{
						IP:   "127.0.0.2",
						Port: "",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "endpoint with TLS enabled",
			cfg: Config{
				Endpoints: []Endpoint{
					{
						IP:         "127.0.0.2",
						Port:       "9090",
						TLSEnabled: true,
					},
				},
			},
			expectErr: false,
		},
		{
			name: "endpoint with X-Token path",
			cfg: Config{
				Endpoints: []Endpoint{
					{
						IP:         "127.0.0.2",
						Port:       "9090",
						TLSEnabled: true,
						XTokenPath: "/path/to/token",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "mixed TLS configurations across endpoints",
			cfg: Config{
				Endpoints: []Endpoint{
					{
						IP:         "127.0.0.2",
						Port:       "9090",
						TLSEnabled: true,
						XTokenPath: "/path/to/token1",
					},
					{
						IP:         "127.0.0.3",
						Port:       "9091",
						TLSEnabled: false,
					},
				},
			},
			expectErr: false,
		},
		{
			name: "legacy endpoint with TLS and endpoints with different TLS settings",
			cfg: Config{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "/path/to/legacy/token",
				Endpoints: []Endpoint{
					{
						IP:         "127.0.0.2",
						Port:       "9091",
						TLSEnabled: false,
					},
				},
			},
			expectErr: false,
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

func TestAllEndpoints(t *testing.T) {
	tests := []struct {
		name          string
		cfg           Config
		expectedLen   int
		includeLegacy bool
		checkTLS      bool
		checkXToken   bool
	}{
		{
			name: "legacy endpoint only",
			cfg: Config{
				IP:   "127.0.0.1",
				Port: "9090",
			},
			expectedLen:   1,
			includeLegacy: true,
		},
		{
			name: "endpoints only",
			cfg: Config{
				Endpoints: []Endpoint{
					{
						IP:   "127.0.0.2",
						Port: "9090",
					},
					{
						IP:   "127.0.0.3",
						Port: "9091",
					},
				},
			},
			expectedLen:   2,
			includeLegacy: false,
		},
		{
			name: "both legacy and endpoints",
			cfg: Config{
				IP:   "127.0.0.1",
				Port: "9090",
				Endpoints: []Endpoint{
					{
						IP:   "127.0.0.2",
						Port: "9090",
					},
				},
			},
			expectedLen:   2,
			includeLegacy: true,
		},
		{
			name:          "no endpoints",
			cfg:           Config{},
			expectedLen:   0,
			includeLegacy: false,
		},
		{
			name: "legacy endpoint with TLS",
			cfg: Config{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: true,
			},
			expectedLen:   1,
			includeLegacy: true,
			checkTLS:      true,
		},
		{
			name: "legacy endpoint with TLS and X-Token",
			cfg: Config{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "/path/to/token",
			},
			expectedLen:   1,
			includeLegacy: true,
			checkTLS:      true,
			checkXToken:   true,
		},
		{
			name: "endpoint with TLS and X-Token",
			cfg: Config{
				Endpoints: []Endpoint{
					{
						IP:         "127.0.0.2",
						Port:       "9090",
						TLSEnabled: true,
						XTokenPath: "/path/to/token",
					},
				},
			},
			expectedLen:   1,
			includeLegacy: false,
			checkTLS:      true,
			checkXToken:   true,
		},
		{
			name: "mixed TLS configurations",
			cfg: Config{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "/path/to/legacy/token",
				Endpoints: []Endpoint{
					{
						IP:         "127.0.0.2",
						Port:       "9091",
						TLSEnabled: false,
					},
					{
						IP:         "127.0.0.3",
						Port:       "9092",
						TLSEnabled: true,
						XTokenPath: "/path/to/other/token",
					},
				},
			},
			expectedLen:   3,
			includeLegacy: true,
			checkTLS:      true,
			checkXToken:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := tt.cfg.GetAllEndpoints()
			require.Equal(t, tt.expectedLen, len(endpoints))

			if tt.includeLegacy && len(endpoints) > 0 {
				// Check if the first endpoint is the legacy one
				require.Equal(t, tt.cfg.IP, endpoints[0].IP)
				require.Equal(t, tt.cfg.Port, endpoints[0].Port)

				if tt.checkTLS {
					// Check that TLS setting is properly preserved
					require.Equal(t, tt.cfg.TLSEnabled, endpoints[0].TLSEnabled)
				}

				if tt.checkXToken {
					// Check that X-Token path is properly preserved
					require.Equal(t, tt.cfg.XTokenPath, endpoints[0].XTokenPath)
				}
			}

			// For non-legacy configs, check each endpoint's settings
			if !tt.includeLegacy && tt.expectedLen > 0 {
				for i, endpoint := range endpoints {
					require.Equal(t, tt.cfg.Endpoints[i].IP, endpoint.IP)
					require.Equal(t, tt.cfg.Endpoints[i].Port, endpoint.Port)

					if tt.checkTLS {
						require.Equal(t, tt.cfg.Endpoints[i].TLSEnabled, endpoint.TLSEnabled)
					}

					if tt.checkXToken {
						require.Equal(t, tt.cfg.Endpoints[i].XTokenPath, endpoint.XTokenPath)
					}
				}
			}

			// For mixed configs, check each endpoint's settings after the legacy one
			if tt.includeLegacy && tt.expectedLen > 1 {
				for i := 1; i < tt.expectedLen; i++ {
					require.Equal(t, tt.cfg.Endpoints[i-1].IP, endpoints[i].IP)
					require.Equal(t, tt.cfg.Endpoints[i-1].Port, endpoints[i].Port)

					if tt.checkTLS {
						require.Equal(t, tt.cfg.Endpoints[i-1].TLSEnabled, endpoints[i].TLSEnabled)
					}

					if tt.checkXToken {
						require.Equal(t, tt.cfg.Endpoints[i-1].XTokenPath, endpoints[i].XTokenPath)
					}
				}
			}
		})
	}
}
