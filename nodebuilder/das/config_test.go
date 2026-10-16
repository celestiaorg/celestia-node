package das

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// TestDefaultConfigSampleTimeout tests that light nodes pin a fixed sample timeout while
// other node types leave it zero so the DASer derives it from the square size.
func TestDefaultConfigSampleTimeout(t *testing.T) {
	light := DefaultConfig(node.Light)
	// the timeout light nodes used before it was derived from the square size
	require.Equal(t, modp2p.BlockTime*time.Duration(light.ConcurrencyLimit), light.SampleTimeout)
	require.NoError(t, light.Validate())

	bridge := DefaultConfig(node.Bridge)
	require.Zero(t, bridge.SampleTimeout)
	require.NoError(t, bridge.Validate())
}
