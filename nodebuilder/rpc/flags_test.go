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
	assert.Equal(t, "Comma-separated list of HTTP methods allowed for CORS (cors enabled default: [GET POST OPTIONS])", corsMethods.Usage)

	// Test corsAllowedHeadersFlag
	corsHeaders := flags.Lookup(corsAllowedHeadersFlag)
	require.NotNil(t, corsHeaders)
	assert.Equal(t, "[]", corsHeaders.Value.String())
	assert.Equal(t, "Comma-separated list of HTTP headers allowed for CORS (cors enabled default: [Content-Type Authorization])", corsHeaders.Usage)
}

// TestParseFlags tests the ParseFlags function in rpc/flags.go
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
		error       bool
	}{
		{
			name:     "addrFlag is set",
			addrFlag: "127.0.0.1:8080",
			portFlag: "",
			expected: &Config{
				Address: "127.0.0.1:8080",
				Port:    "",
				CORS:    CORSConfig{},
			},
			error: false,
		},
		{
			name:     "portFlag is set",
			addrFlag: "",
			portFlag: "9090",
			expected: &Config{
				Address: "",
				Port:    "9090",
				CORS:    CORSConfig{},
			},
			error: false,
		},
		{
			name:     "both addrFlag and portFlag are set",
			addrFlag: "192.168.0.1:1234",
			portFlag: "5678",
			expected: &Config{
				Address: "192.168.0.1:1234",
				Port:    "5678",
				CORS:    CORSConfig{},
			},
			error: false,
		},
		{
			name:     "neither addrFlag nor portFlag are set",
			addrFlag: "",
			portFlag: "",
			expected: &Config{
				Address: "",
				Port:    "",
				CORS:    CORSConfig{},
			},
			error: false,
		},
		{
			name:     "auth flag is set",
			addrFlag: "",
			portFlag: "",
			authFlag: true,
			expected: &Config{
				Address:  "",
				Port:     "",
				SkipAuth: true,
				CORS:     CORSConfig{},
			},
			error: false,
		},
		{
			name:        "CORS enabled with options",
			addrFlag:    "",
			portFlag:    "",
			corsEnabled: true,
			corsOrigins: []string{"http://localhost:3000", "https://example.com"},
			corsMethods: []string{"GET", "POST", "OPTIONS", "PUT", "DELETE"},
			corsHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
			expected: &Config{
				Address: "",
				Port:    "",
				CORS: CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{"http://localhost:3000", "https://example.com"},
					AllowedMethods: []string{"GET", "POST", "OPTIONS", "PUT", "DELETE"},
					AllowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
				},
			},
			error: false,
		},
		{
			name:        "CORS enabled with empty options",
			addrFlag:    "",
			portFlag:    "",
			corsEnabled: true,
			expected: &Config{
				Address: "",
				Port:    "",
				CORS: CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{},
					AllowedMethods: []string{"GET", "POST", "OPTIONS"},
					AllowedHeaders: []string{"Content-Type", "Authorization"},
				},
			},
			error: false,
		},
		{
			name:        "CORS options without CORS enabled",
			addrFlag:    "",
			portFlag:    "",
			corsEnabled: false,
			corsOrigins: []string{"http://localhost:3000", "https://example.com"},
			corsMethods: []string{"GET", "POST", "OPTIONS", "PUT", "DELETE"},
			corsHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
			expected: &Config{
				Address: "",
				Port:    "",
				CORS: CORSConfig{
					Enabled:        false,
					AllowedOrigins: []string{"http://localhost:3000", "https://example.com"},
					AllowedMethods: []string{"GET", "POST", "OPTIONS", "PUT", "DELETE"},
					AllowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
				},
			},
			error: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cfg := &Config{
				CORS: CORSConfig{},
			}
			cmd.Flags().AddFlagSet(Flags())

			err := cmd.Flags().Set(addrFlag, test.addrFlag)
			require.NoError(t, err)

			err = cmd.Flags().Set(portFlag, test.portFlag)
			require.NoError(t, err)

			err = cmd.Flags().Set(authFlag, fmt.Sprintf("%t", test.authFlag))
			require.NoError(t, err)

			err = cmd.Flags().Set(corsEnabledFlag, fmt.Sprintf("%t", test.corsEnabled))
			require.NoError(t, err)

			for _, origin := range test.corsOrigins {
				err = cmd.Flags().Set(corsAllowedOriginsFlag, origin)
				require.NoError(t, err)
			}

			for _, method := range test.corsMethods {
				err = cmd.Flags().Set(corsAllowedMethodsFlag, method)
				require.NoError(t, err)
			}

			for _, header := range test.corsHeaders {
				err = cmd.Flags().Set(corsAllowedHeadersFlag, header)
				require.NoError(t, err)
			}

			err = ParseFlags(cmd, cfg)
			if !test.error {
				assert.NoError(t, err)
				assert.Equal(t, test.expected.Address, cfg.Address)
				assert.Equal(t, test.expected.Port, cfg.Port)
				assert.Equal(t, test.expected.SkipAuth, cfg.SkipAuth)
				assert.Equal(t, test.expected.CORS.Enabled, cfg.CORS.Enabled)
				assert.Equal(t, test.expected.CORS.AllowedOrigins, cfg.CORS.AllowedOrigins)
				assert.Equal(t, test.expected.CORS.AllowedMethods, cfg.CORS.AllowedMethods)
				assert.Equal(t, test.expected.CORS.AllowedHeaders, cfg.CORS.AllowedHeaders)
			} else {
				assert.Error(t, err)
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
