package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	err := Init(dir, DefaultConfig())
	require.NoError(t, err)
	ok, err := IsInit(dir)
	require.NoError(t, err)
	assert.True(t, ok)
}
