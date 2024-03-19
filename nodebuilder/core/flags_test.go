package core

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestParseFlags(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		expected Config
	}{
		{
			name: "Test core.ip flag sets RPC and gRPC hosts",
			args: []string{
				"--core.ip=192.168.1.1",
			},
			expected: Config{
				IP: "192.168.1.1",
				RPC: RPCConfig{
					Host:   "192.168.1.1",
					Port:   DefaultRPCPort,
					Scheme: DefaultRPCScheme,
				},
				GRPC: GRPCConfig{
					Host: "192.168.1.1",
					Port: DefaultGRPCPort,
				},
			},
		},
		{
			name: "Test RPC and gRPC flags",
			args: []string{
				"--core.rpc.host=127.0.0.1",
				"--core.rpc.port=26658",
				"--core.grpc.host=127.0.0.1",
				"--core.grpc.port=9091",
			},
			expected: Config{
				RPC: RPCConfig{
					Host:   "127.0.0.1",
					Port:   "26658",
					Scheme: "http",
				},
				GRPC: GRPCConfig{
					Host: "127.0.0.1",
					Port: "9091",
				},
			},
		},
		{
			name: "Test HTTPS flag",
			args: []string{"--core.rpc.host=127.0.0.1", "--core.rpc.https=true"},
			expected: Config{
				RPC: RPCConfig{
					Host:   "127.0.0.1",
					Port:   DefaultRPCPort, // Assuming DefaultRPCPort is defined elsewhere
					Scheme: "https",
				},
				GRPC: GRPCConfig{
					Host: "",
					Port: DefaultGRPCPort,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup Cobra command
			cmd := &cobra.Command{
				Use: "testCommand",
			}
			cmd.Flags().AddFlagSet(Flags())

			// Apply flags from test case
			err := cmd.ParseFlags(tc.args)
			assert.NoError(t, err)

			// Parse flags into config
			var cfg Config
			err = ParseFlags(cmd, &cfg)
			assert.NoError(t, err)

			// Assert the expected configuration
			assert.Equal(t, tc.expected, cfg)
		})
	}
}
