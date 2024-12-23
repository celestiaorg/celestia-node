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
}

// TestParseFlags tests the ParseFlags function in rpc/flags.go
func TestParseFlags(t *testing.T) {
	tests := []struct {
		name        string
		addrFlag    string
		portFlag    string
		expected    *Config
		shouldError bool
	}{
		{
			name:     "addrFlag is set",
			addrFlag: "127.0.0.1:8080",
			portFlag: "",
			expected: &Config{
				Address: "127.0.0.1:8080",
				Port:    "",
			},
		},
		{
			name:     "portFlag is set",
			addrFlag: "",
			portFlag: "9090",
			expected: &Config{
				Address: "",
				Port:    "9090",
			},
		},
		{
			name:     "both addrFlag and portFlag are set",
			addrFlag: "192.168.0.1:1234",
			portFlag: "5678",
			expected: &Config{
				Address: "192.168.0.1:1234",
				Port:    "5678",
			},
		},
		{
			name:     "neither addrFlag nor portFlag are set",
			addrFlag: "",
			portFlag: "",
			expected: &Config{
				Address: "",
				Port:    "",
			},
		},
		{
			name:     "default values when flags are empty",
			addrFlag: "",
			portFlag: "",
			expected: &Config{
				Address: defaultBindAddress,
				Port:    defaultPort,
			},
		},
		{
			name:        "invalid address format",
			addrFlag:    "invalid:address:format",
			portFlag:    "",
			shouldError: true,
			expected:    &Config{},
		},
		{
			name:        "invalid port number",
			addrFlag:    "",
			portFlag:    "999999",
			shouldError: true,
			expected:    &Config{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cfg := &Config{}

			cmd.Flags().AddFlagSet(Flags())

			if err := cmd.Flags().Set(addrFlag, test.addrFlag); err != nil && !test.shouldError {
				t.Fatalf("unexpected error setting addrFlag: %v", err)
			}
			if err := cmd.Flags().Set(portFlag, test.portFlag); err != nil && !test.shouldError {
				t.Fatalf("unexpected error setting portFlag: %v", err)
			}

			err := ParseFlags(cmd, cfg)
			if test.shouldError {
				assert.Error(t, err)
				return
			}
			
			assert.NoError(t, err)
			assert.Equal(t, test.expected.Address, cfg.Address)
			assert.Equal(t, test.expected.Port, cfg.Port)
		})
	}
}

// TestAddressValidation tests the address validation functionality
func TestAddressValidation(t *testing.T) {
	tests := []struct {
		name    string
		address string
		isValid bool
	}{
		{
			name:    "valid IPv4 address",
			address: "127.0.0.1:8080",
			isValid: true,
		},
		{
			name:    "valid IPv6 address",
			address: "[::1]:8080",
			isValid: true,
		},
		{
			name:    "invalid address format",
			address: "256.256.256.256:99999",
			isValid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{Address: test.address}
			err := cfg.Validate()
			if test.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
