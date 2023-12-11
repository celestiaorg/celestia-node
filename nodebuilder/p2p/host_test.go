package p2p

import (
	"testing"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestUserAgent(t *testing.T) {
	tests := []struct {
		name     string
		net      Network
		build    *node.BuildInfo
		expected string
	}{
		{
			name:     "Testnet",
			net:      "testnet",
			expected: "celestia-node/testnet/v1.0.0/abcdefg",
			build: &node.BuildInfo{
				SemanticVersion: "1.0.0",
				LastCommit:      "abcdefg",
			},
		},
		{
			name:     "Mainnet",
			net:      "mainnet",
			expected: "celestia-node/mainnet/v1.0.0/abcdefg",
			build: &node.BuildInfo{
				SemanticVersion: "1.0.0",
				LastCommit:      "abcdefg",
			},
		},
		{
			name:     "Empty LastCommit",
			net:      "testnet",
			expected: "celestia-node/testnet/unknown/unknown",
			build: &node.BuildInfo{
				SemanticVersion: "",
				LastCommit:      "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userAgent := newUserAgent().WithNetwork(tt.net)
			userAgent.build = tt.build

			if userAgent.String() != tt.expected {
				t.Errorf("Unexpected user agent. Got: %s, want: %s", userAgent, tt.expected)
			}
		})
	}
}
