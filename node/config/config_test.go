package config

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigWriteRead(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	in := DefaultConfig()

	err := WriteTo(in, buf)
	require.NoError(t, err)

	out, err := ReadFrom(buf)
	require.NoError(t, err)
	assert.EqualValues(t, in, out)
}
