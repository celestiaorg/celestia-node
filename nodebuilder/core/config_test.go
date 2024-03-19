package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultValues(t *testing.T) {
	expectedRPCScheme := "http"
	expectedRPCPort := "26657"
	expectedGRPCPort := "9090"

	assert.Equal(t, expectedRPCScheme, DefaultRPCScheme, "DefaultRPCScheme is incorrect")
	assert.Equal(t, expectedRPCPort, DefaultRPCPort, "DefaultRPCPort is incorrect")
	assert.Equal(t, expectedGRPCPort, DefaultGRPCPort, "DefaultGRPCPort is incorrect")
}

func TestGRPCHost(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name: "Fallback to IP when GRPC Host is not set",
			cfg: &Config{
				GRPC: GRPCConfig{
					Host: "",
				},
				IP: "127.0.0.1",
			},
			expected: "127.0.0.1",
		},
		{
			name: "Use GRPC Host when set",
			cfg: &Config{
				GRPC: GRPCConfig{
					Host: "0.0.0.0",
				},
				IP: "127.0.0.1",
			},
			expected: "0.0.0.0",
		},
		// Add more test cases here if needed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.cfg.GRPCHost()
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestRPCHost(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name: "Fallback to IP when GRPC Host is not set",
			cfg: &Config{
				RPC: RPCConfig{
					Host: "",
				},
				IP: "127.0.0.1",
			},
			expected: "127.0.0.1",
		},
		{
			name: "Use GRPC Host when set",
			cfg: &Config{
				RPC: RPCConfig{
					Host: "0.0.0.0",
				},
				IP: "127.0.0.1",
			},
			expected: "0.0.0.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.cfg.RPCHost()
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestMultipleHostsConfigured(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *Config
		expected error
	}{
		{
			name: "IP and RPC Host both configured",
			cfg: &Config{
				IP: "127.0.0.1",
				RPC: RPCConfig{
					Host: "localhost",
				},
			},
			expected: fmt.Errorf("multiple hosts configured: core.ip overridden by core.rpc.host"),
		},
		{
			name: "IP and gRPC Host both configured",
			cfg: &Config{
				IP: "127.0.0.1",
				GRPC: GRPCConfig{
					Host: "localhost",
				},
			},
			expected: fmt.Errorf("multiple hosts configured: core.ip overridden by core.grpc.host"),
		},
		{
			name: "Only IP configured",
			cfg: &Config{
				IP: "127.0.0.1",
			},
			expected: nil,
		},
		{
			name: "Only RPC Host configured",
			cfg: &Config{
				RPC: RPCConfig{
					Host: "localhost",
				},
			},
			expected: nil,
		},
		{
			name: "Only gRPC Host configured",
			cfg: &Config{
				GRPC: GRPCConfig{
					Host: "localhost",
				},
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.cfg.multipleHostsConfigured()
			if tc.expected == nil {
				assert.Nil(t, actual)
			} else {
				assert.Error(t, actual)
				assert.EqualError(t, actual, tc.expected.Error())
			}
		})
	}
}
