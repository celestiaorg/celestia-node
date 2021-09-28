package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsInit(t *testing.T) {
	dir := t.TempDir()
	err := Init(dir)
	require.NoError(t, err)
	ok := IsInit(dir)
	assert.True(t, ok)
}
