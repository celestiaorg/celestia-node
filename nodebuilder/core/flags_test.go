package core

import (
	"testing"

	"github.com/spf13/cobra"
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
				EndpointConfig: EndpointConfig{
					IP:   "",
					Port: "",
				},
			},
			expectError: false,
		},
		{
			name: "only core.ip",
			args: []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{
				EndpointConfig: EndpointConfig{
					Port: DefaultPort,
				},
			},
			expectedCfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: DefaultPort,
				},
			},
			expectError: false,
		},
		{
			name:     "only core.ip, empty port values",
			args:     []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{},
			expectedCfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: DefaultPort,
				},
			},
			expectError: false,
		},
		{
			name: "no flags, values from input config.toml ",
			args: []string{},
			inputCfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.162.36.1",
					Port: "5678",
				},
			},
			expectedCfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.162.36.1",
					Port: "5678",
				},
			},
			expectError: false,
		},
		{
			name: "only core.ip, with config.toml overridden defaults for ports",
			args: []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{
				EndpointConfig: EndpointConfig{
					Port: "5678",
				},
			},
			expectedCfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: "5678", // Config value preserved when flag not changed
				},
			},
			expectError: false,
		},
		{
			name: "core.ip and core.port",
			args: []string{"--core.ip=127.0.0.1", "--core.port=54321"},
			inputCfg: Config{
				EndpointConfig: EndpointConfig{
					Port: DefaultPort,
				},
			},
			expectedCfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: "54321",
				},
			},
			expectError: false,
		},
		{
			name: "core.ip with default core.port",
			args: []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{
				EndpointConfig: EndpointConfig{
					Port: "",
				},
			},
			expectedCfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: DefaultPort,
				},
			},
			expectError: false,
		},
		{
			name: "explicit core.port overrides config",
			args: []string{"--core.ip=127.0.0.1", "--core.port=8080"},
			inputCfg: Config{
				EndpointConfig: EndpointConfig{
					Port: "5678",
				},
			},
			expectedCfg: Config{
				EndpointConfig: EndpointConfig{
					IP:   "127.0.0.1",
					Port: "8080", // Explicit flag overrides config
				},
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
