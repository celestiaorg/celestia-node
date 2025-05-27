package rpc

import (
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlags(t *testing.T) {
	flags := Flags()

	// Test addrFlag
	addr := flags.Lookup(addrFlag)
	require.NotNil(t, addr)
	assert.Equal(t, "", addr.Value.String())
	assert.Equal(t, fmt.Sprintf("Set a custom RPC listen address (default: %s)", defaultBindAddress), addr.Usage)

	// Test portFlag
	port := flags.Lookup(portFlag)
	require.NotNil(t, port)
	assert.Equal(t, "", port.Value.String())
	assert.Equal(t, fmt.Sprintf("Set a custom RPC port (default: %s)", defaultPort), port.Usage)

	// Test authFlag
	auth := flags.Lookup(authFlag)
	require.NotNil(t, auth)
	assert.Equal(t, "false", auth.Value.String())
	assert.Equal(t, "Skips authentication for RPC requests", auth.Usage)

	// Test corsEnabledFlag
	corsEnabled := flags.Lookup(corsEnabledFlag)
	require.NotNil(t, corsEnabled)
	assert.Equal(t, "false", corsEnabled.Value.String())
	assert.Equal(t, "Enable CORS for RPC server", corsEnabled.Usage)

	// Test corsAllowedOriginsFlag
	corsOrigins := flags.Lookup(corsAllowedOriginsFlag)
	require.NotNil(t, corsOrigins)
	assert.Equal(t, "[]", corsOrigins.Value.String())
	assert.Equal(t, "Comma-separated list of origins allowed to access the RPC server via CORS (cors enabled default: empty)", corsOrigins.Usage)

	// Test corsAllowedMethodsFlag
	corsMethods := flags.Lookup(corsAllowedMethodsFlag)
	require.NotNil(t, corsMethods)
	assert.Equal(t, "[]", corsMethods.Value.String())
	assert.Equal(t, fmt.Sprintf("Comma-separated list of HTTP methods allowed for CORS (cors enabled default: %s)", defaultAllowedMethods), corsMethods.Usage)

	// Test corsAllowedHeadersFlag
	corsHeaders := flags.Lookup(corsAllowedHeadersFlag)
	require.NotNil(t, corsHeaders)
	assert.Equal(t, "[]", corsHeaders.Value.String())
	assert.Equal(t, fmt.Sprintf("Comma-separated list of HTTP headers allowed for CORS (cors enabled default: %s)", defaultAllowedHeaders), corsHeaders.Usage)
}

// TestParseFlags tests the core flag parsing functionality
func TestParseFlags(t *testing.T) {
	tests := []struct {
		name        string
		addrFlag    string
		portFlag    string
		authFlag    bool
		corsEnabled bool
		corsOrigins []string
		corsMethods []string
		corsHeaders []string
		expected    *Config
		expectError bool
	}{
		{
			name:     "Basic address and port",
			addrFlag: "127.0.0.1",
			portFlag: "9090",
			expected: &Config{
				Address: "127.0.0.1",
				Port:    "9090",
				CORS:    CORSConfig{},
			},
			expectError: false,
		},
		{
			name:     "Auth disabled",
			authFlag: true,
			expected: &Config{
				Address:  "",
				Port:     "",
				SkipAuth: true,
				CORS:     CORSConfig{},
			},
			expectError: false,
		},
		{
			name:        "CORS enabled with basic config",
			corsEnabled: true,
			corsOrigins: []string{"https://example.com"},
			corsMethods: []string{"GET", "POST"},
			corsHeaders: []string{"Content-Type"},
			expected: &Config{
				Address: "",
				Port:    "",
				CORS: CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{"https://example.com"},
					AllowedMethods: []string{"GET", "POST"},
					AllowedHeaders: []string{"Content-Type"},
				},
			},
			expectError: false,
		},
		{
			name:        "CORS enabled with defaults",
			corsEnabled: true,
			expected: &Config{
				Address: "",
				Port:    "",
				CORS: CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{},
					AllowedMethods: defaultAllowedMethods,
					AllowedHeaders: defaultAllowedHeaders,
				},
			},
			expectError: false,
		},
		{
			name:        "CORS settings without CORS enabled - should error",
			corsEnabled: false,
			corsOrigins: []string{"https://example.com"},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "Wildcard origins allowed",
			corsEnabled: true,
			corsOrigins: []string{"*"},
			corsMethods: []string{"GET", "POST"},
			corsHeaders: []string{"Content-Type"},
			expected: &Config{
				Address: "",
				Port:    "",
				CORS: CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{"*"},
					AllowedMethods: []string{"GET", "POST"},
					AllowedHeaders: []string{"Content-Type"},
				},
			},
			expectError: false,
		},
		{
			name:        "Wildcard headers allowed",
			corsEnabled: true,
			corsOrigins: []string{"https://example.com"},
			corsMethods: []string{"GET", "POST"},
			corsHeaders: []string{"*"},
			expected: &Config{
				Address: "",
				Port:    "",
				CORS: CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{"https://example.com"},
					AllowedMethods: []string{"GET", "POST"},
					AllowedHeaders: []string{"*"},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cfg := &Config{
				CORS: CORSConfig{},
			}
			cmd.Flags().AddFlagSet(Flags())

			err := cmd.Flags().Set(addrFlag, tt.addrFlag)
			require.NoError(t, err)

			err = cmd.Flags().Set(portFlag, tt.portFlag)
			require.NoError(t, err)

			err = cmd.Flags().Set(authFlag, fmt.Sprintf("%t", tt.authFlag))
			require.NoError(t, err)

			err = cmd.Flags().Set(corsEnabledFlag, fmt.Sprintf("%t", tt.corsEnabled))
			require.NoError(t, err)

			for _, origin := range tt.corsOrigins {
				err = cmd.Flags().Set(corsAllowedOriginsFlag, origin)
				require.NoError(t, err)
			}

			for _, method := range tt.corsMethods {
				err = cmd.Flags().Set(corsAllowedMethodsFlag, method)
				require.NoError(t, err)
			}

			for _, header := range tt.corsHeaders {
				err = cmd.Flags().Set(corsAllowedHeadersFlag, header)
				require.NoError(t, err)
			}

			err = ParseFlags(cmd, cfg)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.Address, cfg.Address)
				assert.Equal(t, tt.expected.Port, cfg.Port)
				assert.Equal(t, tt.expected.SkipAuth, cfg.SkipAuth)
				assert.Equal(t, tt.expected.CORS.Enabled, cfg.CORS.Enabled)

				if len(tt.expected.CORS.AllowedOrigins) == 0 {
					assert.Empty(t, cfg.CORS.AllowedOrigins)
				} else {
					assert.Equal(t, tt.expected.CORS.AllowedOrigins, cfg.CORS.AllowedOrigins)
				}

				assert.Equal(t, tt.expected.CORS.AllowedMethods, cfg.CORS.AllowedMethods)
				assert.Equal(t, tt.expected.CORS.AllowedHeaders, cfg.CORS.AllowedHeaders)
			}
		})
	}
}

// TestParseFlagsErrors tests error cases in ParseFlags
func TestParseFlagsErrors(t *testing.T) {
	cmd := &cobra.Command{}
	cfg := &Config{}

	err := ParseFlags(cmd, cfg)
	assert.Error(t, err)
}
