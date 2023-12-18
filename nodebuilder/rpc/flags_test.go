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
		name     string
		addrFlag string
		portFlag string
		expected *Config
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cfg := &Config{}

			cmd.Flags().AddFlagSet(Flags())

			err := cmd.Flags().Set(addrFlag, test.addrFlag)
			if err != nil {
				t.Errorf(err.Error())
			}
			err = cmd.Flags().Set(portFlag, test.portFlag)
			if err != nil {
				t.Errorf(err.Error())
			}

			ParseFlags(cmd, cfg)
			assert.Equal(t, test.expected.Address, cfg.Address)
			assert.Equal(t, test.expected.Port, cfg.Port)
		})
	}
}
