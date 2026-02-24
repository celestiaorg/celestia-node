package p2p

import (
	"testing"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestUserAgent(t *testing.T) {
	tests := []struct {
		name     string
		net      Network
		tp       node.Type
		build    *node.BuildInfo
		expected string
	}{
		{
			name:     "Testnet",
			net:      "testnet",
			tp:       node.Bridge,
			expected: "celestia-node/testnet/bridge/v1.0.0/abcdefg",
			build: &node.BuildInfo{
				SemanticVersion: "v1.0.0",
				LastCommit:      "abcdefg",
			},
		},
		{
			name:     "Mainnet",
			net:      "mainnet",
			expected: "celestia-node/mainnet/light/v1.0.0/abcdefg",
			tp:       node.Light,
			build: &node.BuildInfo{
				SemanticVersion: "v1.0.0",
				LastCommit:      "abcdefg",
			},
		},
		{
			name:     "Empty LastCommit, Empty NodeType",
			net:      "testnet",
			expected: "celestia-node/testnet/unknown/unknown/unknown",
			build: &node.BuildInfo{
				SemanticVersion: "",
				LastCommit:      "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userAgent := newUserAgent().WithNetwork(tt.net).WithNodeType(tt.tp)
			userAgent.build = tt.build

			if userAgent.String() != tt.expected {
				t.Errorf("Unexpected user agent. Got: %s, want: %s", userAgent, tt.expected)
			}
		})
	}
}
