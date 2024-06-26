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
				IP:       "",
				RPCPort:  "",
				GRPCPort: "",
			},
			expectError: false,
		},
		{
			name: "only core.ip",
			args: []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{
				RPCPort:  DefaultRPCPort,
				GRPCPort: DefaultGRPCPort,
			},
			expectedCfg: Config{
				IP:       "127.0.0.1",
				RPCPort:  DefaultRPCPort,
				GRPCPort: DefaultGRPCPort,
			},
			expectError: false,
		},
		{
			name:     "only core.ip, empty port values",
			args:     []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{},
			expectedCfg: Config{
				IP:       "127.0.0.1",
				RPCPort:  DefaultRPCPort,
				GRPCPort: DefaultGRPCPort,
			},
			expectError: true,
		},
		{
			name: "no flags, values from input config.toml ",
			args: []string{},
			inputCfg: Config{
				IP:       "127.162.36.1",
				RPCPort:  "1234",
				GRPCPort: "5678",
			},
			expectedCfg: Config{
				IP:       "127.162.36.1",
				RPCPort:  "1234",
				GRPCPort: "5678",
			},
			expectError: false,
		},
		{
			name: "only core.ip, with config.toml overridden defaults for ports",
			args: []string{"--core.ip=127.0.0.1"},
			inputCfg: Config{
				RPCPort:  "1234",
				GRPCPort: "5678",
			},
			expectedCfg: Config{
				IP:       "127.0.0.1",
				RPCPort:  "1234",
				GRPCPort: "5678",
			},
			expectError: false,
		},
		{
			name: "core.ip and core.rpc.port",
			args: []string{"--core.ip=127.0.0.1", "--core.rpc.port=12345"},
			inputCfg: Config{
				RPCPort:  DefaultRPCPort,
				GRPCPort: DefaultGRPCPort,
			},
			expectedCfg: Config{
				IP:       "127.0.0.1",
				RPCPort:  "12345",
				GRPCPort: DefaultGRPCPort,
			},
			expectError: false,
		},
		{
			name: "core.ip and core.grpc.port",
			args: []string{"--core.ip=127.0.0.1", "--core.grpc.port=54321"},
			inputCfg: Config{
				RPCPort:  DefaultRPCPort,
				GRPCPort: DefaultGRPCPort,
			},
			expectedCfg: Config{
				IP:       "127.0.0.1",
				RPCPort:  DefaultRPCPort,
				GRPCPort: "54321",
			},
			expectError: false,
		},
		{
			name: "core.ip, core.rpc.port, and core.grpc.port",
			args: []string{"--core.ip=127.0.0.1", "--core.rpc.port=12345", "--core.grpc.port=54321"},
			expectedCfg: Config{
				IP:       "127.0.0.1",
				RPCPort:  "12345",
				GRPCPort: "54321",
			},
			expectError: false,
		},
		{
			name:        "core.rpc.port without core.ip",
			args:        []string{"--core.rpc.port=12345"},
			expectedCfg: Config{},
			expectError: true,
		},
		{
			name:        "core.grpc.port without core.ip",
			args:        []string{"--core.grpc.port=54321"},
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
