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
