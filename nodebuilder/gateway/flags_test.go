package gateway

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlags(t *testing.T) {
	flags := Flags()

	enabled := flags.Lookup(enabledFlag)
	require.NotNil(t, enabled)
	assert.Equal(t, "false", enabled.Value.String())
	assert.Equal(t, "Enables the REST gateway", enabled.Usage)

	addr := flags.Lookup(addrFlag)
	require.NotNil(t, addr)
	assert.Equal(t, "", addr.Value.String())
	assert.Equal(t, fmt.Sprintf("Set a custom gateway listen address (default: %s)", defaultBindAddress), addr.Usage)

	port := flags.Lookup(portFlag)
	require.NotNil(t, port)
	assert.Equal(t, "", port.Value.String())
	assert.Equal(t, fmt.Sprintf("Set a custom gateway port (default: %s)", defaultPort), port.Usage)
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name        string
		enabledFlag bool
		addrFlag    string
		portFlag    string
		expectedCfg *Config
	}{
		{
			name:        "Enabled flag is true",
			enabledFlag: true,
			addrFlag:    "127.0.0.1",
			portFlag:    "8080",
			expectedCfg: &Config{
				Enabled: true,
				Address: "127.0.0.1",
				Port:    "8080",
			},
		},
		{
			name:        "Enabled flag is false",
			enabledFlag: false,
			addrFlag:    "127.0.0.1",
			portFlag:    "8080",
			expectedCfg: &Config{
				Enabled: false,
				Address: "127.0.0.1",
				Port:    "8080",
			},
		},
		{
			name:        "Enabled flag is false and address/port flags are not changed",
			enabledFlag: false,
			addrFlag:    "",
			portFlag:    "",
			expectedCfg: &Config{
				Enabled: false,
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

			err := cmd.Flags().Set(enabledFlag, strconv.FormatBool(test.enabledFlag))
			assert.NoError(t, err)
			err = cmd.Flags().Set(addrFlag, test.addrFlag)
			assert.NoError(t, err)
			err = cmd.Flags().Set(portFlag, test.portFlag)
			assert.NoError(t, err)

			ParseFlags(cmd, cfg)
			assert.Equal(t, test.expectedCfg.Enabled, cfg.Enabled)
			assert.Equal(t, test.expectedCfg.Address, cfg.Address)
			assert.Equal(t, test.expectedCfg.Port, cfg.Port)
		})
	}
}
