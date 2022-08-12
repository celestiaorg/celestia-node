package node

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node/config"
)

func TestConfigWriteRead(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	in := config.DefaultConfig(config.Bridge)

	err := in.Encode(buf)
	require.NoError(t, err)

	var out config.Config
	err = out.Decode(buf)
	require.NoError(t, err)
	assert.EqualValues(t, in, &out)
}
