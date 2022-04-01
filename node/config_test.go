//go:build test_unit

package node

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigWriteRead(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	in := DefaultConfig(Bridge)

	err := in.Encode(buf)
	require.NoError(t, err)

	var out Config
	err = out.Decode(buf)
	require.NoError(t, err)
	assert.EqualValues(t, in, &out)
}
