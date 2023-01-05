package nodebuilder

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// TestConfigWriteRead tests that the configs for all node types can be encoded to and from TOML.
func TestConfigWriteRead(t *testing.T) {
	tests := []node.Type{
		node.Full,
		node.Light,
		node.Bridge,
	}

	for _, tp := range tests {
		t.Run(tp.String(), func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			in := DefaultConfig(tp)

			err := in.Encode(buf)
			require.NoError(t, err)

			var out Config
			err = out.Decode(buf)
			require.NoError(t, err)
			assert.EqualValues(t, in.Core, out.Core)
			assert.EqualValues(t, in.State, out.State)
			assert.EqualValues(t, in.P2P, out.P2P)
			assert.EqualValues(t, in.RPC, out.RPC)
			assert.EqualValues(t, in.Gateway, out.Gateway)
			assert.EqualValues(t, in.Share, out.Share)
			assert.EqualValues(t, in.DASer, out.DASer)
		})
	}
}
