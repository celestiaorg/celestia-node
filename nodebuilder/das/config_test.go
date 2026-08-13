package das

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// TestDefaultConfigSampleTimeout tests that light nodes pin a fixed sample timeout while
// other node types leave it zero so the DASer derives it from the square size.
func TestDefaultConfigSampleTimeout(t *testing.T) {
	light := DefaultConfig(node.Light)
	require.NotZero(t, light.SampleTimeout)
	require.NoError(t, light.Validate())

	bridge := DefaultConfig(node.Bridge)
	require.Zero(t, bridge.SampleTimeout)
	require.NoError(t, bridge.Validate())
}
