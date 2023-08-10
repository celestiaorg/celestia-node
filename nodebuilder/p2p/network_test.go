package p2p

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChainIDFromNetwork(t *testing.T) {
	var tests = []struct {
		name            string
		expectedChainID string
	}{
		{name: "mocha-3:custom", expectedChainID: "mocha-3"},
		{name: "mocha-3", expectedChainID: "mocha-3"},
		{name: "celestia:network1", expectedChainID: "celestia"},
		{name: "celestia", expectedChainID: "celestia"},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			require.Equal(t, ChainID(tt.expectedChainID), ChainIDFromNetwork(Network(tt.name)))
		})
	}
}
