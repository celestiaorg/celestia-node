package core

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		inputCfg    Config // config that could be read from ctx
		expectedCfg Config
		expectError bool
	}{
		{
			name:     "no flags",
			args:     []string{},
			inputCfg: Config{},
			expectedCfg: Config{
				IP:   "",
				Port: "",
			},
			expectError: false,
		},
		{
			name: "only core.ip",
			args: []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{
				Port: DefaultPort,
			},
			expectedCfg: Config{
				IP:   "127.0.0.1",
				Port: DefaultPort,
			},
			expectError: false,
		},
		{
			name:     "only core.ip, empty port values",
			args:     []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{},
			expectedCfg: Config{
				IP:   "127.0.0.1",
				Port: DefaultPort,
			},
			expectError: true,
		},
		{
			name: "no flags, values from input config.toml ",
			args: []string{},
			inputCfg: Config{
				IP:   "127.162.36.1",
				Port: "5678",
			},
			expectedCfg: Config{
				IP:   "127.162.36.1",
				Port: "5678",
			},
			expectError: false,
		},
		{
			name: "only core.ip, with config.toml overridden defaults for ports",
			args: []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{
				Port: "5678",
			},
			expectedCfg: Config{
				IP:   "127.0.0.1",
				Port: "5678",
			},
			expectError: false,
		},
		{
			name: "core.ip and core.port",
			args: []string{"--core.ip=127.0.0.1", "--core.port=54321"},
			inputCfg: Config{
				Port: DefaultPort,
			},
			expectedCfg: Config{
				IP:   "127.0.0.1",
				Port: "54321",
			},
			expectError: false,
		},
		{
			name:        "core.port without core.ip",
			args:        []string{"--core.port=54321"},
			expectedCfg: Config{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			flags := Flags()
			cmd.Flags().AddFlagSet(flags)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			require.NoError(t, err)

			err = ParseFlags(cmd, &tt.inputCfg)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedCfg, tt.inputCfg)
			}
		})
	}
}

func TestParseEndpointString(t *testing.T) {
	tests := []struct {
		name                string
		endpointStr         string
		defaultTLS          bool
		defaultXToken       string
		expectedEndpoint    Endpoint
		expectError         bool
		expectedErrorPrefix string
	}{
		{
			name:          "simple endpoint format",
			endpointStr:   "127.0.0.1:9090",
			defaultTLS:    false,
			defaultXToken: "",
			expectedEndpoint: Endpoint{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: false,
				XTokenPath: "",
			},
			expectError: false,
		},
		{
			name:          "simple endpoint with default TLS",
			endpointStr:   "127.0.0.1:9090",
			defaultTLS:    true,
			defaultXToken: "/path/to/token",
			expectedEndpoint: Endpoint{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "/path/to/token",
			},
			expectError: false,
		},
		{
			name:          "endpoint with TLS true",
			endpointStr:   "127.0.0.1:9090:tls=true",
			defaultTLS:    false,
			defaultXToken: "",
			expectedEndpoint: Endpoint{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "",
			},
			expectError: false,
		},
		{
			name:          "endpoint with TLS false",
			endpointStr:   "127.0.0.1:9090:tls=false",
			defaultTLS:    true,
			defaultXToken: "/path/to/token",
			expectedEndpoint: Endpoint{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: false,
				XTokenPath: "/path/to/token",
			},
			expectError: false,
		},
		{
			name:          "endpoint with TLS and X-Token",
			endpointStr:   "127.0.0.1:9090:tls=true:xtoken=/custom/path",
			defaultTLS:    false,
			defaultXToken: "/default/path",
			expectedEndpoint: Endpoint{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "/custom/path",
			},
			expectError: false,
		},
		{
			name:          "endpoint with X-Token only",
			endpointStr:   "127.0.0.1:9090:xtoken=/custom/path",
			defaultTLS:    true,
			defaultXToken: "/default/path",
			expectedEndpoint: Endpoint{
				IP:         "127.0.0.1",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "/custom/path",
			},
			expectError: false,
		},
		{
			name:                "invalid endpoint format",
			endpointStr:         "127.0.0.1",
			defaultTLS:          false,
			defaultXToken:       "",
			expectError:         true,
			expectedErrorPrefix: "invalid endpoint format",
		},
		{
			name:                "invalid TLS value",
			endpointStr:         "127.0.0.1:9090:tls=invalid",
			defaultTLS:          false,
			defaultXToken:       "",
			expectError:         true,
			expectedErrorPrefix: "invalid TLS value",
		},
		{
			name:          "hostname with TLS and X-Token",
			endpointStr:   "consensus.example.com:9090:tls=true:xtoken=/path/to/token",
			defaultTLS:    false,
			defaultXToken: "",
			expectedEndpoint: Endpoint{
				IP:         "consensus.example.com",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "/path/to/token",
			},
			expectError: false,
		},
		{
			name:          "IPv6 address basic",
			endpointStr:   "[2001:db8::1]:9090",
			defaultTLS:    false,
			defaultXToken: "",
			expectedEndpoint: Endpoint{
				IP:         "2001:db8::1",
				Port:       "9090",
				TLSEnabled: false,
				XTokenPath: "",
			},
			expectError: false,
		},
		{
			name:          "IPv6 address with TLS",
			endpointStr:   "[2001:db8::1]:9090:tls=true",
			defaultTLS:    false,
			defaultXToken: "",
			expectedEndpoint: Endpoint{
				IP:         "2001:db8::1",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "",
			},
			expectError: false,
		},
		{
			name:          "IPv6 address with TLS and X-Token",
			endpointStr:   "[2001:db8::1]:9090:tls=true:xtoken=/path/to/token",
			defaultTLS:    false,
			defaultXToken: "",
			expectedEndpoint: Endpoint{
				IP:         "2001:db8::1",
				Port:       "9090",
				TLSEnabled: true,
				XTokenPath: "/path/to/token",
			},
			expectError: false,
		},
		{
			name:                "IPv6 address missing closing bracket",
			endpointStr:         "[2001:db8::1:9090",
			defaultTLS:          false,
			defaultXToken:       "",
			expectError:         true,
			expectedErrorPrefix: "invalid IPv6 address format",
		},
		{
			name:                "IPv6 address missing port",
			endpointStr:         "[2001:db8::1]",
			defaultTLS:          false,
			defaultXToken:       "",
			expectError:         true,
			expectedErrorPrefix: "invalid port format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, err := parseEndpointString(tt.endpointStr, tt.defaultTLS, tt.defaultXToken)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorPrefix != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorPrefix)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedEndpoint, endpoint)
			}
		})
	}
}
